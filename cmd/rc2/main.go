package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

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
	cfg := transfer.DefaultConfig()
	fs.IntVar(&cfg.Window, "window", cfg.Window, "janela em pacotes (1 a 4096)")
	fs.DurationVar(&cfg.RTO, "rto", cfg.RTO, "timeout de retransmissão")
	fs.IntVar(&cfg.Retries, "retries", cfg.Retries, "máximo de retransmissões por pacote")
	fs.DurationVar(&cfg.Idle, "idle", cfg.Idle, "timeout de inatividade do receptor")
	fs.Int64Var(&cfg.MaxSize, "max-size", cfg.MaxSize, "limite do arquivo em bytes")
	if e := fs.Parse(os.Args[2:]); e != nil {
		return e
	}
	if *file == "" && mode != "serve" && mode != "list" {
		return fmt.Errorf("informe -file")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if mode == "serve" || mode == "list" || mode == "upload" || mode == "download" {
		password := os.Getenv("RC2_PASSWORD")
		if password == "" {
			return fmt.Errorf("defina a variável de ambiente RC2_PASSWORD")
		}
		if mode == "serve" {
			c, e := net.ListenPacket("udp", *addr)
			if e != nil {
				return e
			}
			defer c.Close()
			fmt.Fprintln(os.Stderr, "Servidor UDP em", c.LocalAddr(), "- CTRL+C para encerrar")
			return service.Serve(ctx, c, *root, password, cfg)
		}
		a, e := net.ResolveUDPAddr("udp", *addr)
		if e != nil {
			return e
		}
		bind := "0.0.0.0:0"
		if a.IP.To4() == nil {
			bind = "[::]:0"
		}
		c, e := net.ListenPacket("udp", bind)
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
		return output(s, e)
	}
	var s transfer.Stats
	var e error
	if mode == "receive" {
		c, err := net.ListenPacket("udp", *addr)
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
		c, err := net.ListenPacket("udp", bind)
		if err != nil {
			return err
		}
		defer c.Close()
		s, e = transfer.Send(ctx, c, a, *file, cfg)
	}
	return output(s, e)
}
func output(s transfer.Stats, e error) error {
	result := struct {
		Success   bool           `json:"success"`
		Error     string         `json:"error,omitempty"`
		Timestamp string         `json:"timestamp"`
		Metrics   transfer.Stats `json:"metrics"`
	}{Success: e == nil, Timestamp: time.Now().UTC().Format(time.RFC3339), Metrics: s}
	if e != nil {
		result.Error = e.Error()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		return err
	}
	return e
}
