package transfer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublishDoesNotReplace(t *testing.T) {
	dir := t.TempDir()
	src, dst := filepath.Join(dir, "temp"), filepath.Join(dir, "existing")
	if e := os.WriteFile(src, []byte("new"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := os.WriteFile(dst, []byte("original"), 0600); e != nil {
		t.Fatal(e)
	}
	if e := publish(src, dst); e == nil {
		t.Fatal("publication replaced an existing file")
	}
	got, e := os.ReadFile(dst)
	if e != nil || string(got) != "original" {
		t.Fatalf("original modified: %v", e)
	}
}
