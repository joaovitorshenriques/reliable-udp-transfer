// Package service provides authenticated file operations over RC2 UDP packets.
// This first service processes one operation at a time, on a single UDP port.
package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"trabalho_redes/internal/protocol"
	"trabalho_redes/internal/transfer"
)

type Request struct {
	Operation string `json:"operation"`
	Name      string `json:"name,omitempty"`
	Password  string `json:"password"`
	Offset    int    `json:"offset,omitempty"`
}
type Entry struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
}
type Response struct {
	Error string  `json:"error,omitempty"`
	Files []Entry `json:"files,omitempty"`
	Next  int     `json:"next,omitempty"`
}
type cached struct {
	packet protocol.Packet
	until  time.Time
}

// Flat filenames avoid platform-dependent traversal, Windows devices and ADS.
func validName(name string) bool {
	if len(name) == 0 || len(name) > 120 || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("._- ", r)) {
			return false
		}
	}
	stem := strings.ToUpper(strings.Split(name, ".")[0])
	switch stem {
	case "CON", "PRN", "AUX", "NUL":
		return false
	}
	if len(stem) == 4 && (strings.HasPrefix(stem, "COM") || strings.HasPrefix(stem, "LPT")) && stem[3] >= '0' && stem[3] <= '9' {
		return false
	}
	return true
}
func packet(id uint64, r Response) protocol.Packet {
	b, _ := json.Marshal(r)
	return protocol.Packet{Kind: protocol.Response, Session: id, Payload: b}
}
func write(c net.PacketConn, a net.Addr, p protocol.Packet) error {
	b, e := p.Marshal()
	if e != nil {
		return e
	}
	_, e = c.WriteTo(b, a)
	return e
}
func isTimeout(e error) bool { var n net.Error; return errors.As(e, &n) && n.Timeout() }

// boundConn confines the transfer to the authenticated endpoint and repeats the
// control response when the original request is retried after a lost response.
type boundConn struct {
	net.PacketConn
	peer  net.Addr
	reply protocol.Packet
	ctx   context.Context
}

func (b *boundConn) ReadFrom(buf []byte) (int, net.Addr, error) {
	for {
		if e := b.ctx.Err(); e != nil {
			return 0, nil, e
		}
		n, a, e := b.PacketConn.ReadFrom(buf)
		if e != nil {
			return n, a, e
		}
		p, pe := protocol.Parse(buf[:n])
		if a.String() != b.peer.String() {
			if pe == nil && p.Kind == protocol.Request {
				_ = write(b.PacketConn, a, packet(p.Session, Response{Error: "servidor ocupado; tente novamente"}))
			}
			continue
		}
		if pe == nil && p.Kind == protocol.Request {
			if p.Session == b.reply.Session {
				_ = write(b.PacketConn, a, b.reply)
			}
			continue
		}
		return n, a, nil
	}
}

// Serve requires a dedicated directory not modified by untrusted local users.
// Network callers cannot supply directories or follow existing symbolic links.
func Serve(ctx context.Context, c net.PacketConn, root, password string, cfg transfer.Config) error {
	if password == "" {
		return errors.New("defina RC2_PASSWORD com uma senha não vazia")
	}
	if e := cfg.Validate(); e != nil {
		return e
	}
	root, e := filepath.Abs(root)
	if e != nil {
		return e
	}
	if e = os.MkdirAll(root, 0700); e != nil {
		return e
	}
	root, e = filepath.EvalSymlinks(root)
	if e != nil {
		return e
	}
	secret := sha256.Sum256([]byte(password))
	cache := map[string]cached{}
	buf := make([]byte, 1201)
	for {
		if ctx.Err() != nil {
			return nil
		}
		if e = c.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); e != nil {
			return e
		}
		n, a, re := c.ReadFrom(buf)
		if re != nil {
			if isTimeout(re) {
				continue
			}
			if n > 0 {
				continue
			}
			return re
		}
		p, pe := protocol.Parse(buf[:n])
		if pe != nil || p.Kind != protocol.Request {
			continue
		}
		var req Request
		if json.Unmarshal(p.Payload, &req) != nil {
			continue
		}
		supplied := sha256.Sum256([]byte(req.Password))
		if subtle.ConstantTimeCompare(secret[:], supplied[:]) != 1 {
			_ = write(c, a, packet(p.Session, Response{Error: "senha incorreta"}))
			continue
		}
		key := fmt.Sprintf("%s/%d", a.String(), p.Session)
		now := time.Now()
		for k, v := range cache {
			if now.After(v.until) {
				delete(cache, k)
			}
		}
		if v, ok := cache[key]; ok {
			_ = write(c, a, v.packet)
			continue
		}
		if len(cache) >= 128 {
			_ = write(c, a, packet(p.Session, Response{Error: "limite de solicitações; aguarde"}))
			continue
		}
		response := Response{}
		path := ""
		switch req.Operation {
		case "list":
			entries, err := os.ReadDir(root)
			if err != nil {
				response.Error = "não foi possível listar arquivos"
				break
			}
			files := []Entry{}
			for _, v := range entries {
				if validName(v.Name()) && v.Type().IsRegular() {
					info, err := v.Info()
					if err == nil && info.Mode().IsRegular() {
						files = append(files, Entry{v.Name(), info.Size()})
					}
				}
			}
			if req.Offset < 0 || req.Offset > len(files) {
				response.Error = "página inválida"
				break
			}
			end := min(req.Offset+5, len(files))
			response.Files = files[req.Offset:end]
			if end < len(files) {
				response.Next = end
			}
		case "upload", "download":
			if !validName(req.Name) {
				response.Error = "nome inválido: use até 120 caracteres ASCII, sem diretórios"
				break
			}
			path = filepath.Join(root, req.Name)
			info, err := os.Lstat(path)
			if req.Operation == "upload" {
				if !os.IsNotExist(err) {
					response.Error = "destino já existe ou está inacessível"
				}
			} else if err != nil || !info.Mode().IsRegular() {
				response.Error = "arquivo inexistente ou não permitido"
			} else if info.Size() > cfg.MaxSize {
				response.Error = "arquivo excede limite de tamanho"
			}
		default:
			response.Error = "operação desconhecida"
		}
		reply := packet(p.Session, response)
		cache[key] = cached{reply, now.Add(2 * time.Minute)}
		if e = write(c, a, reply); e != nil {
			return e
		}
		if response.Error != "" || req.Operation == "list" {
			continue
		}
		conn := &boundConn{PacketConn: c, peer: a, reply: reply, ctx: ctx}
		var opErr error
		if req.Operation == "upload" {
			_, opErr = transfer.Receive(ctx, conn, path, cfg)
		} else {
			_, opErr = transfer.Send(ctx, conn, a, path, cfg)
		}
		// Never reexecute a completed request: duplicate uploads must not overwrite.
		msg := "solicitação já encerrada; use uma nova solicitação"
		if opErr != nil {
			msg = "transferência encerrada com erro"
			fmt.Fprintf(os.Stderr, "Falha em %s de %s: %v\n", req.Operation, req.Name, opErr)
		}
		cache[key] = cached{packet(p.Session, Response{Error: msg}), time.Now().Add(2 * time.Minute)}
	}
}

func call(ctx context.Context, c net.PacketConn, a net.Addr, req Request, cfg transfer.Config) (Response, error) {
	if e := cfg.Validate(); e != nil {
		return Response{}, e
	}
	var id [8]byte
	if _, e := rand.Read(id[:]); e != nil {
		return Response{}, e
	}
	payload, e := json.Marshal(req)
	if e != nil {
		return Response{}, e
	}
	p := protocol.Packet{Kind: protocol.Request, Session: binary.BigEndian.Uint64(id[:]), Payload: payload}
	buf := make([]byte, 1201)
	for attempt := 0; attempt <= cfg.Retries; attempt++ {
		if e = ctx.Err(); e != nil {
			return Response{}, e
		}
		if e = write(c, a, p); e != nil {
			return Response{}, e
		}
		until := time.Now().Add(cfg.RTO)
		if e = c.SetReadDeadline(until); e != nil {
			return Response{}, e
		}
		for time.Now().Before(until) {
			n, peer, re := c.ReadFrom(buf)
			if re != nil {
				if isTimeout(re) {
					break
				}
				return Response{}, re
			}
			if peer.String() != a.String() {
				continue
			}
			q, err := protocol.Parse(buf[:n])
			if err != nil || q.Kind != protocol.Response || q.Session != p.Session {
				continue
			}
			var r Response
			if json.Unmarshal(q.Payload, &r) != nil {
				continue
			}
			if r.Error != "" {
				return r, errors.New(r.Error)
			}
			return r, nil
		}
	}
	return Response{}, errors.New("servidor não respondeu: limite de tentativas excedido")
}

func List(ctx context.Context, c net.PacketConn, a net.Addr, password string, cfg transfer.Config) ([]Entry, error) {
	files := []Entry{}
	offset := 0
	for {
		r, e := call(ctx, c, a, Request{Operation: "list", Password: password, Offset: offset}, cfg)
		if e != nil {
			return nil, e
		}
		files = append(files, r.Files...)
		if r.Next == 0 {
			return files, nil
		}
		if r.Next <= offset || len(files) > 10000 {
			return nil, errors.New("paginação inválida ou limite de listagem excedido")
		}
		offset = r.Next
	}
}
func Upload(ctx context.Context, c net.PacketConn, a net.Addr, password, local, remote string, cfg transfer.Config) (transfer.Stats, error) {
	info, e := os.Stat(local)
	if e != nil {
		return transfer.Stats{}, e
	}
	if !info.Mode().IsRegular() || info.Size() > cfg.MaxSize {
		return transfer.Stats{}, errors.New("arquivo local inválido ou muito grande")
	}
	if _, e = call(ctx, c, a, Request{Operation: "upload", Name: remote, Password: password}, cfg); e != nil {
		return transfer.Stats{}, e
	}
	return transfer.Send(ctx, c, a, local, cfg)
}
func Download(ctx context.Context, c net.PacketConn, a net.Addr, password, remote, local string, cfg transfer.Config) (transfer.Stats, error) {
	if _, e := os.Lstat(local); !os.IsNotExist(e) {
		return transfer.Stats{}, errors.New("destino local já existe ou está inacessível")
	}
	if _, e := os.Stat(filepath.Dir(local)); e != nil {
		return transfer.Stats{}, e
	}
	if _, e := call(ctx, c, a, Request{Operation: "download", Name: remote, Password: password}, cfg); e != nil {
		return transfer.Stats{}, e
	}
	// Only accept START from the server that authenticated the request.
	conn := &boundConn{PacketConn: c, peer: a, ctx: ctx}
	return transfer.Receive(ctx, conn, local, cfg)
}
