package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"trabalho_redes/internal/cdn"
	"trabalho_redes/internal/netio"
	"trabalho_redes/internal/service"
	"trabalho_redes/internal/transfer"
)

//go:embed index.html
var page []byte

func main() {
	port := flag.Int("port", 8081, "porta HTTP local")
	flag.Parse()
	addr := fmt.Sprintf("127.0.0.1:%d", *port)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(page)
	})
	cfg := transfer.DefaultConfig()
	busy := make(chan struct{}, 1)
	// Reuse UDP endpoints across operations so the service can reuse its leases.
	sockets := map[string]net.PacketConn{}
	mux.HandleFunc("POST /api/{operation}", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Origin") != "http://"+addr || r.Header.Get("X-RC2-Request") != "1" {
			http.Error(w, "Origem inválida.", http.StatusForbidden)
			return
		}
		select {
		case busy <- struct{}{}:
			defer func() { <-busy }()
		default:
			http.Error(w, "Já há uma operação em andamento.", http.StatusConflict)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, cfg.MaxSize+1<<20)
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			http.Error(w, "Formulário inválido ou arquivo grande demais.", http.StatusBadRequest)
			return
		}
		defer r.MultipartForm.RemoveAll()
		ctx := r.Context()
		password := r.FormValue("password")
		if password == "" || len(password) > 256 {
			http.Error(w, "Informe uma senha de até 256 bytes.", http.StatusBadRequest)
			return
		}
		operation := r.PathValue("operation")
		peer, conn, err := endpoint(r.FormValue("address"), sockets)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		switch operation {
		case "list":
			files, err := service.List(ctx, conn, peer, password, cfg)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadGateway)
				return
			}
			sendJSON(w, files)
		case "probe":
			second, secondConn, err := endpoint(r.FormValue("replica"), sockets)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			selected, results, err := cdn.Select(ctx, []string{peer.String(), second.String()}, 5,
				func(ctx context.Context, address string) (time.Duration, error) {
					if address == peer.String() {
						return service.Probe(ctx, conn, peer, password, time.Second)
					}
					return service.Probe(ctx, secondConn, second, password, time.Second)
				})
			report := map[string]any{"selected": selected, "results": results}
			if err != nil {
				report["error"] = err.Error()
			}
			sendJSON(w, report)
		case "upload", "download":
			handleTransfer(w, r, conn, peer, password, cfg, operation)
		default:
			http.NotFound(w, r)
		}
	})
	server := &http.Server{
		Addr: addr, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 30 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; connect-src 'self'; frame-ancestors 'none'")
			if r.Host != addr {
				http.Error(w, "Endereço HTTP inválido.", http.StatusForbidden)
				return
			}
			mux.ServeHTTP(w, r)
		}),
	}
	log.Printf("Interface RC2 em http://%s", addr)
	log.Fatal(server.ListenAndServe())
}

func endpoint(address string, sockets map[string]net.PacketConn) (*net.UDPAddr, net.PacketConn, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(address))
	if err != nil || net.ParseIP(host) == nil {
		return nil, nil, fmt.Errorf("Informe um IP e uma porta UDP, como 127.0.0.1:9000.")
	}
	peer, err := net.ResolveUDPAddr("udp", net.JoinHostPort(host, port))
	if err != nil || peer.Port == 0 || peer.IP.IsUnspecified() || peer.IP.IsMulticast() {
		return nil, nil, fmt.Errorf("Endereço UDP inválido.")
	}
	bind := "0.0.0.0:0"
	if peer.IP.To4() == nil {
		bind = "[::]:0"
	}
	if conn := sockets[bind]; conn != nil {
		return peer, conn, nil
	}
	conn, err := netio.Listen(bind, 4*1024*1024)
	if err == nil {
		sockets[bind] = conn
	}
	return peer, conn, err
}

func handleTransfer(w http.ResponseWriter, r *http.Request, conn net.PacketConn, peer *net.UDPAddr, password string, cfg transfer.Config, operation string) {
	dir, err := os.MkdirTemp("", "rc2-web-")
	if err != nil {
		http.Error(w, "Falha ao criar diretório temporário.", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "transfer.bin")
	ctx := r.Context()
	name := r.FormValue("name")
	if operation == "upload" {
		file, header, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Selecione um arquivo.", http.StatusBadRequest)
			return
		}
		defer file.Close()
		if name == "" {
			name = header.Filename
		}
		if header.Size > cfg.MaxSize {
			http.Error(w, "Arquivo acima do limite.", http.StatusRequestEntityTooLarge)
			return
		}
		dst, err := os.Create(path)
		if err != nil {
			http.Error(w, "Falha ao preparar o arquivo.", http.StatusInternalServerError)
			return
		}
		_, err = io.Copy(dst, file)
		closeErr := dst.Close()
		if err != nil || closeErr != nil {
			http.Error(w, "Falha ao gravar o arquivo temporário.", http.StatusInternalServerError)
			return
		}
		stats, err := service.Upload(ctx, conn, peer, password, path, name, cfg)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		sendJSON(w, stats)
		return
	}
	stats, err := service.Download(ctx, conn, peer, password, name, path, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		http.Error(w, "Falha ao abrir o arquivo recebido.", http.StatusInternalServerError)
		return
	}
	defer f.Close()
	metrics, _ := json.Marshal(stats)
	w.Header().Set("X-RC2-Metrics", string(metrics))
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	http.ServeContent(w, r, name, time.Time{}, f)
}

func sendJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
