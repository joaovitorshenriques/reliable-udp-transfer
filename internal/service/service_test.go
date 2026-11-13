package service

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trabalho_redes/internal/protocol"
	"trabalho_redes/internal/transfer"
)

func udp(t *testing.T) net.PacketConn {
	t.Helper()
	c, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

// Drop the first successful response to exercise request replay during transfer.
type loseReply struct {
	net.PacketConn
	dropped bool
}

func (c *loseReply) WriteTo(b []byte, a net.Addr) (int, error) {
	p, e := protocol.Parse(b)
	if e == nil && p.Kind == protocol.Response && !c.dropped {
		c.dropped = true
		return len(b), nil
	}
	return c.PacketConn.WriteTo(b, a)
}

func setup(t *testing.T, drop bool) (context.Context, net.PacketConn, net.Addr, string, transfer.Config) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	root := t.TempDir()
	server := udp(t)
	if drop {
		server = &loseReply{PacketConn: server}
	}
	cfg := transfer.DefaultConfig()
	cfg.MaxRTO = 500 * time.Millisecond
	cfg.RTO = 50 * time.Millisecond
	cfg.Retries = 4
	cfg.Idle = time.Second
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, server, root, "test-password", cfg) }()
	t.Cleanup(func() {
		cancel()
		select {
		case e := <-done:
			if e != nil {
				t.Errorf("server: %v", e)
			}
		case <-time.After(2 * time.Second):
			t.Error("server failed to stop")
		}
	})
	return ctx, udp(t), server.LocalAddr(), root, cfg
}
func TestAuthenticationAndList(t *testing.T) {
	ctx, c, a, root, cfg := setup(t, false)
	for i := 0; i < 13; i++ {
		if e := os.WriteFile(filepath.Join(root, fmt.Sprintf("file%02d.txt", i)), []byte("hello"), 0600); e != nil {
			t.Fatal(e)
		}
	}
	if e := os.WriteFile(filepath.Join(root, ".hidden.part"), []byte("secret"), 0600); e != nil {
		t.Fatal(e)
	}
	for _, password := range []string{"", "wrong"} {
		if _, e := List(ctx, c, a, password, cfg); e == nil || !strings.Contains(e.Error(), "senha incorreta") {
			t.Fatalf("expected authentication rejection, got %v", e)
		}
	}
	files, e := List(ctx, c, a, "test-password", cfg)
	if e != nil || len(files) != 13 {
		t.Fatalf("list: %v %v", files, e)
	}
}
func TestUploadDownload(t *testing.T) {
	for _, op := range []string{"upload", "download"} {
		t.Run(op, func(t *testing.T) {
			ctx, c, a, root, cfg := setup(t, true)
			local := filepath.Join(t.TempDir(), "local.bin")
			remote := filepath.Join(root, "remote.bin")
			content := bytes.Repeat([]byte("UDP transfer test\n"), 10000)
			source, dest := local, remote
			if op == "download" {
				source, dest = remote, local
			}
			if e := os.WriteFile(source, content, 0600); e != nil {
				t.Fatal(e)
			}
			var e error
			if op == "upload" {
				_, e = Upload(ctx, c, a, "test-password", local, "remote.bin", cfg)
			} else {
				_, e = Download(ctx, c, a, "test-password", "remote.bin", local, cfg)
			}
			if e != nil {
				t.Fatal(e)
			}
			got, e := os.ReadFile(dest)
			if e != nil || !bytes.Equal(got, content) {
				t.Fatalf("content mismatch %v", e)
			}
		})
	}
}
func TestInvalidOperations(t *testing.T) {
	ctx, c, a, root, cfg := setup(t, false)
	if e := os.WriteFile(filepath.Join(root, "existing.txt"), []byte("keep"), 0600); e != nil {
		t.Fatal(e)
	}
	cases := []Request{{Operation: "download", Name: "missing.txt"}, {Operation: "upload", Name: "existing.txt"}, {Operation: "download", Name: "../outside"}, {Operation: "upload", Name: "C:\\outside"}, {Operation: "download", Name: "existing.txt:secret"}, {Operation: "unknown"}, {Operation: "list", Offset: -1}}
	for _, req := range cases {
		req.Password = "test-password"
		if _, e := call(ctx, c, a, req, cfg); e == nil || strings.Contains(e.Error(), "não respondeu") {
			t.Fatalf("accepted %+v", req)
		}
	}
	b, _ := os.ReadFile(filepath.Join(root, "existing.txt"))
	if string(b) != "keep" {
		t.Fatal("existing changed")
	}
}
func TestFileNames(t *testing.T) {
	for _, s := range []string{"../x", "a/b", "a\\b", ".hidden", "CON", "aux.txt", "NUL", "COM1.log", "x:y", "file.", "file ", ""} {
		if validName(s) {
			t.Errorf("accepted %q", s)
		}
	}
	if !validName("my-file_01.bin") {
		t.Fatal("ordinary filename rejected")
	}
}
