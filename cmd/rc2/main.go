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
		return fmt.Errorf("uso: rc2 send|receive [opções]; use -h para ajuda")
	}
	mode := os.Args[1]
	if mode != "send" && mode != "receive" {
		return fmt.Errorf("comando inválido: %s", mode)
	}
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:9000", "destino (send) ou endereço local (receive)")
	file := fs.String("file", "", "arquivo de entrada (send) ou saída nova (receive)")
	cfg := transfer.DefaultConfig()
	fs.IntVar(&cfg.Window, "window", cfg.Window, "janela em pacotes (1 a 4096)")
	fs.DurationVar(&cfg.RTO, "rto", cfg.RTO, "timeout de retransmissão")
	fs.IntVar(&cfg.Retries, "retries", cfg.Retries, "máximo de retransmissões por pacote")
	fs.DurationVar(&cfg.Idle, "idle", cfg.Idle, "timeout de inatividade do receptor")
	fs.Int64Var(&cfg.MaxSize, "max-size", cfg.MaxSize, "limite do arquivo em bytes")
	if e := fs.Parse(os.Args[2:]); e != nil {
		return e
	}
	if *file == "" {
		return fmt.Errorf("informe -file")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
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
