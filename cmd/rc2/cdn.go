package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"
	"trabalho_redes/internal/cdn"
	"trabalho_redes/internal/netio"
	"trabalho_redes/internal/service"
	"trabalho_redes/internal/transfer"
)

func runCDN(args []string) error {
	fs := flag.NewFlagSet("cdn", flag.ContinueOnError)
	servers := fs.String("servers", "", "comma-separated replica UDP addresses")
	passwordFile := fs.String("password-file", "", "UTF-8 password file")
	samples := fs.Int("samples", 5, "probe samples per replica (3..15)")
	timeout := fs.Duration("probe-timeout", time.Second, "single probe timeout")
	name := fs.String("name", "", "remote replicated filename")
	file := fs.String("file", "", "new local destination")
	probeOnly := fs.Bool("probe-only", false, "select replica without downloading")
	selection := fs.String("selection", "", "new JSON selection report file")
	cfg := transfer.DefaultConfig()
	fs.IntVar(&cfg.Window, "window", cfg.Window, "transfer window")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if *timeout < time.Millisecond || *timeout > 10*time.Second {
		return fmt.Errorf("probe timeout must be 1ms..10s")
	}
	if !*probeOnly && (*file == "" || *name == "") {
		return fmt.Errorf("provide -file and -name")
	}
	if !*probeOnly {
		if _, err := os.Lstat(*file); !os.IsNotExist(err) {
			return fmt.Errorf("destination exists or is inaccessible")
		}
	}
	password := os.Getenv("RC2_PASSWORD")
	if *passwordFile != "" {
		b, err := os.ReadFile(*passwordFile)
		if err != nil {
			return err
		}
		password = strings.TrimRight(strings.TrimPrefix(string(b), "\ufeff"), "\r\n")
	}
	if password == "" {
		return fmt.Errorf("provide password file or RC2_PASSWORD")
	}
	addresses := strings.Split(*servers, ",")
	peers := map[string]*net.UDPAddr{}
	for i, a := range addresses {
		peer, err := net.ResolveUDPAddr("udp", strings.TrimSpace(a))
		if err != nil {
			return err
		}
		if peer.IP == nil || peer.IP.IsUnspecified() || peer.Port == 0 {
			return fmt.Errorf("replica requires a concrete host and port")
		}
		addresses[i] = peer.String()
		peers[peer.String()] = peer
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	// Reuse one socket per replica to avoid exhausting server endpoint leases.
	connections := map[string]net.PacketConn{}
	defer func() {
		for _, c := range connections {
			c.Close()
		}
	}()
	probe := func(ctx context.Context, address string) (time.Duration, error) {
		c := connections[address]
		if c == nil {
			bind := "0.0.0.0:0"
			if peers[address].IP.To4() == nil {
				bind = "[::]:0"
			}
			var err error
			c, err = netio.Listen(bind, 4*1024*1024)
			if err != nil {
				return 0, err
			}
			connections[address] = c
		}
		return service.Probe(ctx, c, peers[address], password, *timeout)
	}
	selected, results, err := cdn.Select(ctx, addresses, *samples, probe)
	report := struct {
		Timestamp string       `json:"timestamp"`
		Selected  string       `json:"selected"`
		Results   []cdn.Result `json:"results"`
		Error     string       `json:"error,omitempty"`
	}{Timestamp: time.Now().UTC().Format(time.RFC3339), Selected: selected, Results: results}
	if err != nil {
		report.Error = err.Error()
	}
	if *selection != "" {
		f, e := os.OpenFile(*selection, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		e = json.NewEncoder(f).Encode(report)
		closeErr := f.Close()
		if e != nil {
			return e
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if e := json.NewEncoder(os.Stdout).Encode(report); e != nil {
		return e
	}
	if err != nil || *probeOnly {
		return err
	}
	stats, err := service.Download(ctx, connections[selected], peers[selected], password, *name, *file, cfg)
	r := service.Report{Success: err == nil, Timestamp: time.Now().UTC().Format(time.RFC3339), Endpoint: "client", Operation: "cdn-download", Name: *name, Metrics: stats}
	if err != nil {
		r.Error = err.Error()
	}
	if e := json.NewEncoder(os.Stdout).Encode(r); e != nil {
		return e
	}
	return err
}
