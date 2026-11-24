package main

import (
	"bufio"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"trabalho_redes/internal/netem"
	"trabalho_redes/internal/service"
)

//go:embed index.html
var page []byte

type transferRecord struct {
	service.Report
	Group      string `json:"group"`
	Repetition int    `json:"repetition"`
}

type lossRecord struct {
	Group          string      `json:"group"`
	Repetition     int         `json:"repetition"`
	Window         int         `json:"window_packets"`
	ClientToServer netem.Stats `json:"client_to_server"`
	ServerToClient netem.Stats `json:"server_to_client"`
}

// Keep recent rows in a bounded ring. An unfinished line is reread on the next refresh.
func recent[T any](path string, valid func(T) bool) ([]T, int, error) {
	const limit = 200
	rows := make([]T, 0, limit)
	if path == "" {
		return rows, 0, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return rows, 0, err
	}
	defer f.Close()
	total, skipped := 0, 0
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		line := strings.TrimSpace(strings.TrimPrefix(scan.Text(), "\ufeff"))
		if line == "" {
			continue
		}
		var row T
		if json.Unmarshal([]byte(line), &row) != nil || !valid(row) {
			skipped++
			continue
		}
		if len(rows) < limit {
			rows = append(rows, row)
		} else {
			rows[total%limit] = row
		}
		total++
	}
	if total > limit {
		offset := total % limit
		rows = append(append(make([]T, 0, limit), rows[offset:]...), rows[:offset]...)
	}
	return rows, skipped, scan.Err()
}

func main() {
	metrics := flag.String("metrics", "results/dashboard.jsonl", "JSONL de transferências")
	emulation := flag.String("emulation", "", "JSONL de contadores do emulador (opcional)")
	port := flag.Int("port", 8080, "porta HTTP local")
	flag.Parse()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	mux.HandleFunc("GET /api/metrics", func(w http.ResponseWriter, r *http.Request) {
		transfers, skipped, err := recent(*metrics, func(t transferRecord) bool { return t.Operation != "" })
		loss, skippedLoss, lossErr := recent(*emulation, func(t lossRecord) bool { return t.Group != "" })
		warnings := []string{}
		if os.IsNotExist(err) {
			warnings = append(warnings, "Aguardando o arquivo de métricas da primeira transferência.")
		} else if err != nil {
			warnings = append(warnings, "Falha ao ler o arquivo de métricas.")
		}
		if lossErr != nil {
			warnings = append(warnings, "Falha ao ler o arquivo do emulador.")
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{"transfers": transfers, "loss": loss, "warnings": warnings, "skipped": skipped + skippedLoss})
	})
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	server := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second, WriteTimeout: 15 * time.Second}
	log.Printf("Dashboard: http://%s", addr)
	log.Fatal(server.ListenAndServe())
}
