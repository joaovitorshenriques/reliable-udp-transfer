package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"trabalho_redes/internal/service"
)

func TestStage4Scenarios(t *testing.T) {
	g := scenarios(16, 42)
	for i, want := range []struct {
		rtt  time.Duration
		loss float64
	}{{25 * time.Millisecond, 0}, {100 * time.Millisecond, 0}, {100 * time.Millisecond, 0.05}, {400 * time.Millisecond, 0.05}} {
		if g[i].RTT != want.rtt || g[i].Loss != want.loss || g[i].Window != 16 || !g[i].Adaptive {
			t.Fatalf("scenario %d: %+v", i, g[i])
		}
	}
}

// Opt-in integration for real UDP; uses 1 MiB, not benchmark evidence.
func TestStage4UDP(t *testing.T) {
	if os.Getenv("RC2_STAGE4_TEST") != "1" {
		t.Skip("set RC2_STAGE4_TEST=1 or validate -stage4")
	}
	for _, g := range scenarios(64, 2026) {
		t.Run(g.Name, func(t *testing.T) {
			work := t.TempDir()
			source := filepath.Join(work, "source.bin")
			hash, err := generate(source, 1)
			if err != nil {
				t.Fatal(err)
			}
			var reports []service.Report
			var emulator emulationRecord
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			err = pairWithEmulation(ctx, work, source, hash, g, func(r service.Report) error { reports = append(reports, r); return nil }, func(e emulationRecord) error { emulator = e; return nil })
			if err != nil {
				t.Fatal(err)
			}
			if len(reports) != 4 {
				t.Fatalf("reports=%d", len(reports))
			}
			for _, r := range reports {
				if !r.Success || r.Metrics.SHA256 != hash {
					t.Fatalf("bad report %+v", r)
				}
			}
			if emulator.ClientToServer.Forwarded == 0 || emulator.ServerToClient.Forwarded == 0 {
				t.Fatal("missing emulator counters")
			}
			if g.Loss > 0 && emulator.ClientToServer.RandomDrops+emulator.ServerToClient.RandomDrops == 0 {
				t.Fatal("loss not exercised")
			}
		})
	}
}
