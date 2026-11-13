package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestGeneratedFileIntegrity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "input.bin")
	expected, err := generate(path, 1)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() != 1<<20 {
		t.Fatalf("size=%d", info.Size())
	}
	actual, err := hashFile(path)
	if err != nil || actual != expected {
		t.Fatalf("hash=%s want=%s error=%v", actual, expected, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, err = f.WriteAt([]byte{255}, 0)
	f.Close()
	if err != nil {
		t.Fatal(err)
	}
	actual, err = hashFile(path)
	if err != nil || actual == expected {
		t.Fatalf("corruption not detected: %v", err)
	}
}

func TestInvalidExperimentDoesNotCreateOutput(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results")
	for _, tc := range [][2]int{{0, 80}, {101, 80}, {5, 0}, {5, 1025}} {
		if err := run(context.Background(), path, tc[0], tc[1]); err == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("output created: %v", err)
	}
}
