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
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
	"trabalho_redes/internal/netem"

	"trabalho_redes/internal/netio"
	"trabalho_redes/internal/service"
	"trabalho_redes/internal/transfer"
)

type group struct {
	Name     string
	Adaptive bool
	Buffer   int
	RTT      time.Duration
	Loss     float64
	Window   int
	Seed     int64
}

var groups = []group{{Name: "A"}, {Name: "B", Buffer: 4 * 1024 * 1024}, {Name: "C", Adaptive: true, Buffer: 4 * 1024 * 1024}}

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
	stage4 := flag.Bool("stage4", false, "run the four RTT/loss scenarios instead of stage 3 A/B/C")
	window := flag.Int("window", 64, "stage 4 window in packets")
	scenario := flag.Int("scenario", 0, "stage 4 scenario 1..4, or 0 for all")
	seed := flag.Int64("seed", 2026, "stage 4 base loss seed")
	flag.Parse()
	if *stage4 {
		if *window < 1 || *window > 4096 || *scenario < 0 || *scenario > 4 {
			fmt.Fprintln(os.Stderr, "invalid window/scenario")
			os.Exit(1)
		}
		groups = scenarios(*window, *seed)
		if *scenario > 0 {
			groups = groups[*scenario-1 : *scenario]
		}
	}
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
	prefix := "etapa3-"
	if groups[0].RTT > 0 {
		prefix = "etapa4-"
	}
	dir, err := os.MkdirTemp(parent, prefix)
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
	}{time.Now().UTC().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH, runtime.Version(), repeats, mib, hash, groups, transfer.DefaultConfig(), "loopback; client/server in one process; rotated group order; Config durations in ns; stage4 overrides window and adds RTT/2 per direction, Bernoulli loss for ALL datagrams including ACK/control; queues 8192 each; client seed=group seed+1000*repetition, server seed=client seed+1; pair timeout 2h; generated seeds reproduce decisions only for the same packet order"}
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
	emulationFile, err := os.Create(filepath.Join(dir, "emulation.jsonl"))
	if err != nil {
		return err
	}
	defer emulationFile.Close()
	cw := csv.NewWriter(csvFile)
	if err = cw.Write([]string{"group", "repetition", "requested_udp_buffer", "operation", "endpoint", "role", "session", "success", "error", "bytes", "seconds", "goodput_mbps", "data_retransmissions", "mean_data_rtt_ms", "sha256", "sender_rc2_mbps"}); err != nil {
		return err
	}
	save := func(g group, rep int, r service.Report) error {
		if err := json.NewEncoder(jsonFile).Encode(record{g.Name, rep, g.Buffer, r}); err != nil {
			return err
		}
		s := r.Metrics
		f := func(v float64) string { return strconv.FormatFloat(v, 'g', -1, 64) }
		throughput := ""
		if r.Success && s.Role == "sender" && s.Seconds > 0 {
			throughput = f(float64(s.TransportBytesSent) * 8 / s.Seconds / 1e6)
		}
		if err := cw.Write([]string{g.Name, strconv.Itoa(rep), strconv.Itoa(g.Buffer), r.Operation, r.Endpoint, s.Role, s.Session, strconv.FormatBool(r.Success), r.Error, strconv.FormatInt(s.Bytes, 10), f(s.Seconds), f(s.GoodputMbps), strconv.Itoa(s.Retransmissions), f(s.RTTMillis), s.SHA256, throughput}); err != nil {
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
			g.Seed += int64(rep) * 1000
			fmt.Printf("Grupo %s, repetição %d/%d: upload + download de %d MiB\n", g.Name, rep, repeats, mib)
			if err := pairWithEmulation(ctx, work, source, hash, g, func(r service.Report) error { return save(g, rep, r) }, func(e emulationRecord) error {
				e.Repetition = rep
				if err := json.NewEncoder(emulationFile).Encode(e); err != nil {
					return err
				}
				return emulationFile.Sync()
			}); err != nil {
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
	return pairWithEmulation(parent, work, source, expected, g, save, nil)
}
func pairWithEmulation(parent context.Context, work, source, expected string, g group, save func(service.Report) error, saveEmulation func(emulationRecord) error) (resultErr error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Hour)
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
	var clientConn, serverConn net.PacketConn = client, server
	if g.RTT > 0 {
		ec, err := netem.Wrap(client, netem.Config{Delay: g.RTT / 2, Loss: g.Loss, Seed: g.Seed, Queue: 8192})
		if err != nil {
			return err
		}
		es, err := netem.Wrap(server, netem.Config{Delay: g.RTT / 2, Loss: g.Loss, Seed: g.Seed + 1, Queue: 8192})
		if err != nil {
			ec.Close()
			return err
		}
		clientConn, serverConn = ec, es
		defer func() {
			ec.Close()
			es.Close()
			e := emulationRecord{Group: g.Name, RTTMillis: float64(g.RTT) / float64(time.Millisecond), Loss: g.Loss, Seed: g.Seed, Window: g.Window, ClientToServer: ec.Snapshot(), ServerToClient: es.Snapshot()}
			if saveEmulation != nil {
				if err := saveEmulation(e); err != nil && resultErr == nil {
					resultErr = err
				}
			}
			if resultErr == nil && (e.ClientToServer.QueueDrops+e.ServerToClient.QueueDrops+e.ClientToServer.SendErrors+e.ServerToClient.SendErrors) > 0 {
				resultErr = fmt.Errorf("uncontrolled queue drops/send errors; inspect emulation.jsonl")
			}
		}()
	}
	cfg := transfer.DefaultConfig()
	cfg.Adaptive = g.Adaptive
	if g.Window > 0 {
		cfg.Window = g.Window
	}
	reports := make(chan service.Report, 2)
	done := make(chan struct{})
	var serverErr error
	go func() {
		serverErr = service.ServeWithReporter(ctx, serverConn, serverRoot, "local-experiment", cfg, func(r service.Report) error { reports <- r; return nil })
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
			s, opErr = service.Upload(ctx, clientConn, server.LocalAddr(), "local-experiment", source, "remote.bin", cfg)
		} else {
			s, opErr = service.Download(ctx, clientConn, server.LocalAddr(), "local-experiment", "remote.bin", dest, cfg)
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
