package service

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net"
	"os"
	"sync"
	"time"

	"trabalho_redes/internal/protocol"
	"trabalho_redes/internal/transfer"
)

// Each endpoint owns a bounded queue and a single protocol worker. There is
// only one reader of the real socket; read deadlines are local to each worker.
const MaxClients = 8
const clientQueueSize = 256

type peerConn struct {
	net.PacketConn
	peer     net.Addr
	packets  chan []byte
	closed   chan struct{}
	once     sync.Once
	deadline time.Time
	last     time.Time
	lease    time.Duration
}

func (p *peerConn) Close() error                      { p.once.Do(func() { close(p.closed) }); return nil }
func (p *peerConn) SetReadDeadline(t time.Time) error { p.deadline = t; return nil }
func (p *peerConn) ReadFrom(b []byte) (int, net.Addr, error) {
	if time.Since(p.last) >= p.lease {
		return 0, nil, net.ErrClosed
	}
	until := p.last.Add(p.lease)
	if !p.deadline.IsZero() && p.deadline.Before(until) {
		until = p.deadline
	}
	timer := time.NewTimer(time.Until(until))
	defer timer.Stop()
	select {
	case <-p.closed:
		return 0, nil, net.ErrClosed
	case <-timer.C:
		return 0, nil, os.ErrDeadlineExceeded
	case packet := <-p.packets:
		p.last = time.Now()
		return copy(b, packet), p.peer, nil
	}
}

// ServeWithReporter serves up to MaxClients authenticated endpoints concurrently.
// One endpoint still performs one operation at a time. Reports are serialized.
func ServeWithReporter(ctx context.Context, c net.PacketConn, root, password string, cfg transfer.Config, report func(Report) error) error {
	if password == "" {
		return errors.New("defina RC2_PASSWORD com uma senha não vazia")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	var reportMu sync.Mutex
	clients := map[string]*peerConn{}
	finished := make(chan *peerConn, MaxClients)
	failures := make(chan error, MaxClients)
	defer func() {
		cancel()
		for _, p := range clients {
			p.Close()
		}
		wg.Wait()
	}()
	secret := sha256.Sum256([]byte(password))
	lease := max(2*time.Minute, cfg.Idle, time.Duration(cfg.Retries+2)*max(cfg.RTO, cfg.MaxRTO)) + time.Second
	buf := make([]byte, 1201)
	for {
		if ctx.Err() != nil {
			return nil
		}
		select {
		case err := <-failures:
			return err
		default:
		}
		// Reap before routing so a stale worker cannot consume a new request.
		for {
			select {
			case p := <-finished:
				if clients[p.peer.String()] == p {
					delete(clients, p.peer.String())
				}
			default:
				goto read
			}
		}
	read:
		if err := c.SetReadDeadline(time.Now().Add(100 * time.Millisecond)); err != nil {
			return err
		}
		n, a, err := c.ReadFrom(buf)
		if err != nil {
			if isTimeout(err) || n > 0 {
				continue
			}
			return err
		}
		key := a.String()
		p := clients[key]
		if p == nil {
			q, err := protocol.Parse(buf[:n])
			if err != nil || q.Kind != protocol.Request {
				continue
			}
			var req Request
			if json.Unmarshal(q.Payload, &req) != nil {
				continue
			}
			supplied := sha256.Sum256([]byte(req.Password))
			if subtle.ConstantTimeCompare(secret[:], supplied[:]) != 1 {
				_ = write(c, a, packet(q.Session, Response{Error: "senha incorreta"}))
				continue
			}
			if len(clients) >= MaxClients {
				_ = write(c, a, packet(q.Session, Response{Error: "limite de clientes; tente novamente"}))
				continue
			}
			p = &peerConn{PacketConn: c, peer: a, packets: make(chan []byte, clientQueueSize), closed: make(chan struct{}), last: time.Now(), lease: lease}
			clients[key] = p
			wg.Add(1)
			go func(p *peerConn) {
				defer wg.Done()
				var reporter func(Report) error
				if report != nil {
					reporter = func(r Report) error { reportMu.Lock(); defer reportMu.Unlock(); return report(r) }
				}
				err := servePeer(ctx, p, root, password, cfg, reporter)
				p.Close()
				if err != nil && !errors.Is(err, net.ErrClosed) && ctx.Err() == nil {
					failures <- err
				}
				finished <- p
			}(p)
		}
		select {
		case p.packets <- append([]byte(nil), buf[:n]...):
		default: /* bounded drop; transport retries */
		}
	}
}
