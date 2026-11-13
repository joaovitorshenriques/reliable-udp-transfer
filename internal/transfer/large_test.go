package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"
	"trabalho_redes/internal/netio"
)

func TestLargeFile(t *testing.T) {
	if os.Getenv("RC2_LARGE_TEST") != "1" {
		t.Skip("set RC2_LARGE_TEST=1 to transfer 80 MiB")
	}
	dir := t.TempDir()
	in, out := filepath.Join(dir, "original.bin"), filepath.Join(dir, "received.bin")
	f, e := os.Create(in)
	if e != nil {
		t.Fatal(e)
	}
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = byte(i*13 + i/251)
	}
	for i := 0; i < 80; i++ {
		if _, e = f.Write(chunk); e != nil {
			f.Close()
			t.Fatal(e)
		}
	}
	if e = f.Close(); e != nil {
		t.Fatal(e)
	}
	recv, send := socket(t), socket(t)
	if os.Getenv("RC2_LARGE_BUFFERS") == "1" {
		var e error
		recv, e = netio.Listen("127.0.0.1:0", 4*1024*1024)
		if e != nil {
			t.Fatal(e)
		}
		defer recv.Close()
		send, e = netio.Listen("127.0.0.1:0", 4*1024*1024)
		if e != nil {
			t.Fatal(e)
		}
		defer send.Close()
	}
	cfg := DefaultConfig()
	cfg.Idle = 30 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := Receive(ctx, recv, out, cfg); done <- e }()
	s, e := Send(ctx, send, recv.LocalAddr(), in, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if e = <-done; e != nil {
		t.Fatal(e)
	}
	result, e := os.Open(out)
	if e != nil {
		t.Fatal(e)
	}
	defer result.Close()
	h := sha256.New()
	n, e := io.Copy(h, result)
	if e != nil {
		t.Fatal(e)
	}
	if n != 80*(1<<20) || hex.EncodeToString(h.Sum(nil)) != s.SHA256 {
		t.Fatal("large file integrity failed")
	}
	t.Logf("bytes=%d sha256=%s duration=%.3fs goodput=%.3fMbps retransmissions=%d", n, s.SHA256, s.Seconds, s.GoodputMbps, s.Retransmissions)
}
