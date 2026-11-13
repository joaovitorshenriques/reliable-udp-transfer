// Command validate runs the same checks on Windows and Linux using only Go.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func main() {
	large := flag.Bool("large", false, "include the 80 MiB loopback transfer")
	cross := flag.Bool("cross", false, "also compile for the other OS (does not execute its tests)")
	flag.Parse()
	if err := run(*large, *cross); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(large, cross bool) error {
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run from the repository root: %w", err)
	}
	goName := "go"
	if runtime.GOOS == "windows" {
		goName += ".exe"
	}
	goTool := filepath.Join(runtime.GOROOT(), "bin", goName)
	// Build outputs are temporary, so validation never replaces a running server.
	out, err := os.MkdirTemp("", "rc2-validation-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(out)
	check := func(target string, args ...string) error {
		fmt.Printf("\n[%s/%s] go %v\n", target, runtime.GOARCH, args)
		cmd := exec.Command(goTool, args...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(), "GOOS="+target, "GOARCH="+runtime.GOARCH, "GOTOOLCHAIN=local", "GOPROXY=off", "RC2_LARGE_TEST=0")
		if large {
			cmd.Env = append(cmd.Env, "RC2_LARGE_TEST=1", "RC2_LARGE_BUFFERS=1")
		}
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("validation failed: %w", err)
		}
		return nil
	}
	for _, args := range [][]string{
		{"test", "./...", "-count=1", "-v", "-timeout=5m"},
		{"vet", "./..."},
		{"build", "-o", out + string(os.PathSeparator), "./cmd/..."},
	} {
		if err := check(runtime.GOOS, args...); err != nil {
			return err
		}
	}
	if cross {
		target := "linux"
		if runtime.GOOS == "linux" {
			target = "windows"
		}
		if err := check(target, "build", "./..."); err != nil {
			return err
		}
		fmt.Printf("Cross-compilation for %s passed (build only; tests executed on %s).\n", target, runtime.GOOS)
	}
	return nil
}
