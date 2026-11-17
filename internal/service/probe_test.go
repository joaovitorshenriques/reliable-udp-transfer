package service

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestProbeAuthenticationAndReuse(t *testing.T) {
	ctx, c, a, root, cfg := setup(t, false)
	if _, err := Probe(ctx, c, a, "wrong", time.Second); err == nil {
		t.Fatal("accepted wrong password")
	}
	for i := 0; i < 5; i++ {
		if d, err := Probe(ctx, c, a, "test-password", time.Second); err != nil || d < 0 {
			t.Fatalf("probe %d: %v %v", i, d, err)
		}
	}
	if _, err := List(ctx, c, a, "test-password", cfg); err != nil {
		t.Fatal(err)
	}
	payload := []byte("replicated content")
	if err := os.WriteFile(filepath.Join(root, "replica.bin"), payload, 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "received.bin")
	if _, err := Download(ctx, c, a, "test-password", "replica.bin", target, cfg); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(got, payload) {
		t.Fatalf("download mismatch: %v", err)
	}
}
