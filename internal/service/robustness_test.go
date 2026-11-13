package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"trabalho_redes/internal/protocol"
	"trabalho_redes/internal/transfer"
)

func TestInterruptedUploadRecovery(t *testing.T) {
	for _, mode := range []string{"idle", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			server, client, other := udp(t), udp(t), udp(t)
			root := t.TempDir()
			cfg := transfer.DefaultConfig()
			cfg.Adaptive = false
			cfg.RTO = 50 * time.Millisecond
			cfg.Idle = 400 * time.Millisecond
			cfg.Retries = 3
			reports := make(chan Report, 4)
			done := make(chan error, 1)
			go func() {
				done <- ServeWithReporter(ctx, server, root, "test", cfg, func(r Report) error { reports <- r; return nil })
			}()
			defer func() {
				cancel()
				select {
				case e := <-done:
					if e != nil {
						t.Error(e)
					}
				case <-time.After(time.Second):
					t.Error("server did not stop")
				}
			}()
			if _, err := call(ctx, client, server.LocalAddr(), Request{Operation: "upload", Name: "partial.bin", Password: "test"}, cfg); err != nil {
				t.Fatal(err)
			}
			meta, _ := json.Marshal(map[string]any{"size": 2 * protocol.PayloadSize, "sha256": strings.Repeat("0", 64)})
			start := protocol.Packet{Kind: protocol.Start, Session: 77, Window: 4, Payload: meta}
			ack := func(p protocol.Packet) {
				t.Helper()
				if err := write(client, server.LocalAddr(), p); err != nil {
					t.Fatal(err)
				}
				if err := client.SetReadDeadline(time.Now().Add(time.Second)); err != nil {
					t.Fatal(err)
				}
				buf := make([]byte, 1201)
				n, _, err := client.ReadFrom(buf)
				if err != nil {
					t.Fatal(err)
				}
				q, err := protocol.Parse(buf[:n])
				if err != nil || q.Kind != protocol.Ack || q.Session != 77 || q.Seq != p.Seq {
					t.Fatalf("ACK: %+v %v", q, err)
				}
			}
			ack(start)
			data := protocol.Packet{Kind: protocol.Data, Session: 77, Seq: 1, Payload: make([]byte, protocol.PayloadSize)}
			ack(data)
			// Another endpoint stays available; the active endpoint cannot start
			// a second operation before completing its current upload.
			if files, err := List(ctx, other, server.LocalAddr(), "test", cfg); err != nil || len(files) != 0 {
				t.Fatalf("other client must remain available: %v %v", files, err)
			}
			if _, err := List(ctx, client, server.LocalAddr(), "test", cfg); err == nil || !strings.Contains(err.Error(), "ocupado") {
				t.Fatalf("same client: %v", err)
			}
			if mode == "cancel" {
				cancel()
			}
			// Replays must not keep an abandoned partial upload alive forever.
			ticker := time.NewTicker(30 * time.Millisecond)
			defer ticker.Stop()
			deadline := time.NewTimer(2 * time.Second)
			defer deadline.Stop()
		wait:
			for {
				select {
				case r := <-reports:
					if r.Success || r.Metrics.Bytes != protocol.PayloadSize {
						t.Fatalf("failure report: %+v", r)
					}
					break wait
				case <-ticker.C:
					if mode == "idle" {
						_ = write(client, server.LocalAddr(), start)
						_ = write(client, server.LocalAddr(), data)
					}
				case <-deadline.C:
					t.Fatal("interrupted transfer did not terminate")
				}
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("partial file leaked: %v %v", entries, err)
			}
			if mode == "cancel" {
				return
			}
			local := filepath.Join(t.TempDir(), "source")
			if err := os.WriteFile(local, []byte("recovered"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Upload(ctx, other, server.LocalAddr(), "test", local, "good.bin", cfg); err != nil {
				t.Fatal(err)
			}
			select {
			case r := <-reports:
				if !r.Success {
					t.Fatalf("recovery: %+v", r)
				}
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			files, err := List(ctx, other, server.LocalAddr(), "test", cfg)
			if err != nil || len(files) != 1 || files[0].Name != "good.bin" {
				t.Fatalf("list after recovery: %v %v", files, err)
			}
		})
	}
}
