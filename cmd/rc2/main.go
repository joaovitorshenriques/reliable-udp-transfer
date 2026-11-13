package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"trabalho_redes/internal/netio"
	"trabalho_redes/internal/service"
	"trabalho_redes/internal/transfer"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "Erro:", e)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("uso: rc2 serve|list|upload|download|send|receive [opções]")
	}
	mode := os.Args[1]
	if mode != "send" && mode != "receive" && mode != "serve" && mode != "list" && mode != "upload" && mode != "download" {
		return fmt.Errorf("comando inválido: %s", mode)
	}
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:9000", "destino (send) ou endereço local (receive)")
	file := fs.String("file", "", "arquivo de entrada (send) ou saída nova (receive)")
	root := fs.String("dir", "data/server", "pasta de arquivos do servidor")
	name := fs.String("name", "", "nome remoto para upload/download")
	metricsPath := fs.String("metrics", "", "arquivo JSONL de métricas (append)")
	passwordFile := fs.String("password-file", "", "arquivo local UTF-8 com a senha; alternativa a RC2_PASSWORD")
	socketBuffer := fs.Int("udp-buffer", 4*1024*1024, "buffer UDP solicitado em bytes; 0 mantém padrão do sistema")
	cfg := transfer.DefaultConfig()
	fs.IntVar(&cfg.Window, "window", cfg.Window, "janela em pacotes (1 a 4096)")
	fs.DurationVar(&cfg.RTO, "rto", cfg.RTO, "RTO inicial dos dados e fixo das mensagens de controle")
	fs.BoolVar(&cfg.Adaptive, "adaptive", cfg.Adaptive, "adaptar timeout dos dados ao RTT")
	fs.DurationVar(&cfg.MinRTO, "min-rto", cfg.MinRTO, "piso do RTO adaptativo")
	fs.DurationVar(&cfg.MaxRTO, "max-rto", cfg.MaxRTO, "teto do RTO adaptativo")
	fs.IntVar(&cfg.Retries, "retries", cfg.Retries, "máximo de retransmissões por pacote")
	fs.DurationVar(&cfg.Idle, "idle", cfg.Idle, "timeout de inatividade do receptor")
	fs.Int64Var(&cfg.MaxSize, "max-size", cfg.MaxSize, "limite do arquivo em bytes")
	if e := fs.Parse(os.Args[2:]); e != nil {
		return e
	}
	if *file == "" && mode != "serve" && mode != "list" {
		return fmt.Errorf("informe -file")
	}
	if e := cfg.Validate(); e != nil {
		return e
	}
	var metricsFile *os.File
	if *metricsPath != "" {
		if e := os.MkdirAll(filepath.Dir(*metricsPath), 0700); e != nil {
			return e
		}
		var e error
		metricsFile, e = os.OpenFile(*metricsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return e
		}
		defer metricsFile.Close()
	}
	record := func(r service.Report) error {
		if metricsFile != nil {
			if e := json.NewEncoder(metricsFile).Encode(r); e != nil {
				return e
			}
			if e := metricsFile.Sync(); e != nil {
				return e
			}
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	}
	finish := func(s transfer.Stats, e error) error {
		r := service.Report{Success: e == nil, Timestamp: time.Now().UTC().Format(time.RFC3339), Endpoint: "client", Operation: mode, Name: *name, Metrics: s}
		if e != nil {
			r.Error = e.Error()
		}
		if re := record(r); re != nil {
			return re
		}
		return e
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if mode == "serve" || mode == "list" || mode == "upload" || mode == "download" {
		password := os.Getenv("RC2_PASSWORD")
		if *passwordFile != "" {
			data, err := os.ReadFile(*passwordFile)
			if err != nil {
				return err
			}
			password = strings.TrimRight(strings.TrimPrefix(string(data), "\ufeff"), "\r\n")
		}
		if password == "" {
			return fmt.Errorf("defina a variável de ambiente RC2_PASSWORD")
		}
		if mode == "serve" {
			c, e := netio.Listen(*addr, *socketBuffer)
			if e != nil {
				return e
			}
			defer c.Close()
			fmt.Fprintln(os.Stderr, "Servidor UDP em", c.LocalAddr(), "- CTRL+C para encerrar")
			return service.ServeWithReporter(ctx, c, *root, password, cfg, record)
		}
		a, e := net.ResolveUDPAddr("udp", *addr)
		if e != nil {
			return e
		}
		bind := "0.0.0.0:0"
		if a.IP.To4() == nil {
			bind = "[::]:0"
		}
		c, e := netio.Listen(bind, *socketBuffer)
		if e != nil {
			return e
		}
		defer c.Close()
		if mode == "list" {
			files, e := service.List(ctx, c, a, password, cfg)
			if e != nil {
				return e
			}
			return json.NewEncoder(os.Stdout).Encode(files)
		}
		if *name == "" {
			return fmt.Errorf("informe -name com o nome remoto")
		}
		var s transfer.Stats
		if mode == "upload" {
			s, e = service.Upload(ctx, c, a, password, *file, *name, cfg)
		} else {
			s, e = service.Download(ctx, c, a, password, *name, *file, cfg)
		}
		return finish(s, e)
	}
	var s transfer.Stats
	var e error
	if mode == "receive" {
		c, err := netio.Listen(*addr, *socketBuffer)
		if err != nil {
			return err
		}
		defer c.Close()
		fmt.Fprintln(os.Stderr, "Aguardando UMA transferência em", c.LocalAddr(), "(sem autenticação; use rede de teste)")
		s, e = transfer.Receive(ctx, c, *file, cfg)
	} else {
		a, err := net.ResolveUDPAddr("udp", *addr)
		if err != nil {
			return err
		}
		bind := "0.0.0.0:0"
		if a.IP.To4() == nil {
			bind = "[::]:0"
		}
		c, err := netio.Listen(bind, *socketBuffer)
		if err != nil {
			return err
		}
		defer c.Close()
		s, e = transfer.Send(ctx, c, a, *file, cfg)
	}
	return finish(s, e)
}
