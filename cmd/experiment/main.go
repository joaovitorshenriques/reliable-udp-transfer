// Command experiment compares stage 3 configurations over local UDP.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"trabalho_redes/internal/netio"
	"trabalho_redes/internal/service"
	"trabalho_redes/internal/transfer"
)

type group struct {
	Name     string
	Adaptive bool
	Buffer   int
}

var groups = []group{{"A", false, 0}, {"B", false, 4 * 1024 * 1024}, {"C", true, 4 * 1024 * 1024}}

type record struct {
	Group              string `json:"group"`
	Repetition         int    `json:"repetition"`
	RequestedUDPBuffer int    `json:"requested_udp_buffer"`
	service.Report
}

func main() {
	repeats := flag.Int("repeats", 5, "repetitions per group")
	mib := flag.Int("mib", 80, "file size in MiB (smaller sizes are smoke tests only)")
	output := flag.String("out", "results", "parent directory for a new unique run")
	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if err := run(ctx, *output, *repeats, *mib); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, parent string, repeats, mib int) error {
	if repeats < 1 || repeats > 100 || mib < 1 || mib > 1024 {
		return fmt.Errorf("repeats must be 1..100 and mib 1..1024")
	}
	if err := os.MkdirAll(parent, 0700); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(parent, "etapa3-")
	if err != nil {
		return err
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	fmt.Println("Resultados:", abs)
	// Only generated payloads are temporary; reports survive failures and cancellation.
	work, err := os.MkdirTemp("", "rc2-experiment-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	source := filepath.Join(work, "source.bin")
	hash, err := generate(source, mib)
	if err != nil {
		return err
	}
	manifest := struct {
		StartedUTC          string
		OS, Arch, GoVersion string
		Repetitions, MiB    int
		SHA256              string
		Groups              []group
		Config              transfer.Config
		Scope               string
	}{time.Now().UTC().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH, runtime.Version(), repeats, mib, hash, groups, transfer.DefaultConfig(), "loopback; client/server goroutines in one process; no injected loss; group order rotated by repetition; durations in Config are nanoseconds"}
	b, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(dir, "manifest.json"), b, 0600); err != nil {
		return err
	}
	jsonFile, err := os.Create(filepath.Join(dir, "metrics.jsonl"))
	if err != nil {
		return err
	}
	defer jsonFile.Close()
	csvFile, err := os.Create(filepath.Join(dir, "metrics.csv"))
	if err != nil {
		return err
	}
	defer csvFile.Close()
	cw := csv.NewWriter(csvFile)
	if err = cw.Write([]string{"group", "repetition", "requested_udp_buffer", "operation", "endpoint", "role", "session", "success", "error", "bytes", "seconds", "goodput_mbps", "data_retransmissions", "mean_data_rtt_ms", "sha256"}); err != nil {
		return err
	}
	save := func(g group, rep int, r service.Report) error {
		if err := json.NewEncoder(jsonFile).Encode(record{g.Name, rep, g.Buffer, r}); err != nil {
			return err
		}
		s := r.Metrics
		f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
		if err := cw.Write([]string{g.Name, strconv.Itoa(rep), strconv.Itoa(g.Buffer), r.Operation, r.Endpoint, s.Role, s.Session, strconv.FormatBool(r.Success), r.Error, strconv.FormatInt(s.Bytes, 10), f(s.Seconds), f(s.GoodputMbps), strconv.Itoa(s.Retransmissions), f(s.RTTMillis), s.SHA256}); err != nil {
			return err
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return err
		}
		if err := jsonFile.Sync(); err != nil {
			return err
		}
		return csvFile.Sync()
	}
	for rep := 1; rep <= repeats; rep++ {
		for offset := range groups {
			g := groups[(rep-1+offset)%len(groups)]
			fmt.Printf("Grupo %s, repetição %d/%d: upload + download de %d MiB\n", g.Name, rep, repeats, mib)
			if err := pair(ctx, work, source, hash, g, func(r service.Report) error { return save(g, rep, r) }); err != nil {
				return fmt.Errorf("group %s repetition %d: %w (partial results: %s)", g.Name, rep, err, abs)
			}
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "COMPLETE"), []byte("All uploads/downloads and destination hashes verified.\n"), 0600); err != nil {
		return err
	}
	fmt.Println("Campanha concluída. Métricas e manifesto em", abs)
	return nil
}

func pair(parent context.Context, work, source, expected string, g group, save func(service.Report) error) error {
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	root, err := os.MkdirTemp(work, "pair-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(root)
	serverRoot := filepath.Join(root, "server")
	if err = os.Mkdir(serverRoot, 0700); err != nil {
		return err
	}
	server, err := netio.Listen("127.0.0.1:0", g.Buffer)
	if err != nil {
		return err
	}
	defer server.Close()
	client, err := netio.Listen("127.0.0.1:0", g.Buffer)
	if err != nil {
		return err
	}
	defer client.Close()
	cfg := transfer.DefaultConfig()
	cfg.Adaptive = g.Adaptive
	reports := make(chan service.Report, 2)
	done := make(chan struct{})
	var serverErr error
	go func() {
		serverErr = service.ServeWithReporter(ctx, server, serverRoot, "local-experiment", cfg, func(r service.Report) error { reports <- r; return nil })
		close(done)
	}()
	// Wait for the worker before removing its files, also on an early failure.
	defer func() { cancel(); <-done }()
	for _, op := range []string{"upload", "download"} {
		var s transfer.Stats
		var opErr error
		dest := filepath.Join(root, "download.bin")
		if op == "upload" {
			dest = filepath.Join(serverRoot, "remote.bin")
			s, opErr = service.Upload(ctx, client, server.LocalAddr(), "local-experiment", source, "remote.bin", cfg)
		} else {
			s, opErr = service.Download(ctx, client, server.LocalAddr(), "local-experiment", "remote.bin", dest, cfg)
		}
		r := service.Report{Success: opErr == nil, Timestamp: time.Now().UTC().Format(time.RFC3339), Endpoint: "client", Operation: op, Name: "remote.bin", Metrics: s}
		if opErr != nil {
			r.Error = opErr.Error()
		}
		if err := save(r); err != nil {
			return err
		}
		if opErr != nil {
			return opErr
		}
		select {
		case remote := <-reports:
			if err := save(remote); err != nil {
				return err
			}
			if !remote.Success || s.Session == "" || remote.Metrics.Session != s.Session || remote.Metrics.SHA256 != expected || s.SHA256 != expected || remote.Metrics.Role == s.Role {
				return fmt.Errorf("invalid paired reports for %s", op)
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-done:
			return fmt.Errorf("server stopped before reporting %s: %v", op, serverErr)
		}
		actual, err := hashFile(dest)
		if err != nil {
			return err
		}
		if actual != expected {
			return fmt.Errorf("destination hash mismatch for %s", op)
		}
		fmt.Printf("  %s: hash OK, cliente/%s %.3fs, %.3f Mbit/s\n", op, s.Role, s.Seconds, s.GoodputMbps)
	}
	return nil
}

func generate(path string, mib int) (string, error) {
	f, err := os.Create(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	chunk := make([]byte, 1<<20)
	for i := range chunk {
		chunk[i] = byte(i*13 + i/251)
	}
	for i := 0; i < mib; i++ {
		if _, err := io.MultiWriter(f, h).Write(chunk); err != nil {
			return "", err
		}
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
