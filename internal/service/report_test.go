package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"trabalho_redes/internal/transfer"
)

func TestServerReportsBothRoles(t *testing.T) {
	for _, op := range []string{"upload", "download"} {
		t.Run(op, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			server, client := udp(t), udp(t)
			root := t.TempDir()
			local := filepath.Join(t.TempDir(), "local")
			source := local
			if op == "download" {
				source = filepath.Join(root, "file.bin")
			}
			if e := os.WriteFile(source, []byte("metrics payload"), 0600); e != nil {
				t.Fatal(e)
			}
			cfg := transfer.DefaultConfig()
			cfg.RTO = 50 * time.Millisecond
			cfg.Retries = 2
			reports := make(chan Report, 1)
			done := make(chan error, 1)
			go func() {
				done <- ServeWithReporter(ctx, server, root, "test", cfg, func(r Report) error { reports <- r; return nil })
			}()
			defer func() {
				cancel()
				if e := <-done; e != nil {
					t.Error(e)
				}
			}()
			var s transfer.Stats
			var e error
			if op == "upload" {
				s, e = Upload(ctx, client, server.LocalAddr(), "test", local, "file.bin", cfg)
			} else {
				s, e = Download(ctx, client, server.LocalAddr(), "test", "file.bin", local, cfg)
			}
			if e != nil {
				t.Fatal(e)
			}
			select {
			case r := <-reports:
				if !r.Success || r.Operation != op || r.Endpoint != "server" || r.Metrics.Session != s.Session || r.Metrics.SHA256 != s.SHA256 || r.Metrics.Role == s.Role {
					t.Fatalf("bad paired report: %+v client=%+v", r, s)
				}
				if r.Metrics.TransportBytesSent != s.TransportBytesReceived || r.Metrics.TransportBytesReceived != s.TransportBytesSent {
					t.Fatalf("traffic accounting mismatch: %+v %+v", r.Metrics, s)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("missing server report")
			}
		})
	}
}
