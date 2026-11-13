package service

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"trabalho_redes/internal/transfer"
)

// In-memory datagrams exercise dispatch and deadlines even where host UDP is
// restricted. The same scenario also runs over real UDP in the standard suite.
type memoryPacket struct {
	data []byte
	from net.Addr
}
type memoryNet struct {
	mu    sync.Mutex
	peers map[string]*memoryConn
}
type memoryConn struct {
	network  *memoryNet
	addr     net.Addr
	inbox    chan memoryPacket
	closed   chan struct{}
	once     sync.Once
	deadline time.Time
}

func (n *memoryNet) listen(t *testing.T) net.PacketConn {
	n.mu.Lock()
	defer n.mu.Unlock()
	c := &memoryConn{network: n, addr: &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 10000 + len(n.peers)}, inbox: make(chan memoryPacket, 1024), closed: make(chan struct{})}
	n.peers[c.addr.String()] = c
	t.Cleanup(func() { c.Close() })
	return c
}
func (c *memoryConn) LocalAddr() net.Addr               { return c.addr }
func (c *memoryConn) Close() error                      { c.once.Do(func() { close(c.closed) }); return nil }
func (c *memoryConn) SetReadDeadline(d time.Time) error { c.deadline = d; return nil }
func (c *memoryConn) SetWriteDeadline(time.Time) error  { return nil }
func (c *memoryConn) SetDeadline(d time.Time) error     { return c.SetReadDeadline(d) }
func (c *memoryConn) WriteTo(b []byte, a net.Addr) (int, error) {
	c.network.mu.Lock()
	p := c.network.peers[a.String()]
	c.network.mu.Unlock()
	if p == nil {
		return len(b), nil
	}
	select {
	case p.inbox <- memoryPacket{append([]byte(nil), b...), c.addr}:
	default:
	}
	return len(b), nil
}
func (c *memoryConn) ReadFrom(b []byte) (int, net.Addr, error) {
	timer := time.NewTimer(time.Until(c.deadline))
	defer timer.Stop()
	select {
	case <-c.closed:
		return 0, nil, net.ErrClosed
	case <-timer.C:
		return 0, nil, os.ErrDeadlineExceeded
	case p := <-c.inbox:
		return copy(b, p.data), p.from, nil
	}
}

func TestConcurrentClients(t *testing.T) {
	for _, mode := range []string{"memory", "udp"} {
		t.Run(mode, func(t *testing.T) {
			listen := udp
			if mode == "memory" {
				n := &memoryNet{peers: map[string]*memoryConn{}}
				listen = n.listen
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server := listen(t)
			root := t.TempDir()
			cfg := transfer.DefaultConfig()
			cfg.RTO = 50 * time.Millisecond
			cfg.Retries = 3
			done := make(chan error, 1)
			go func() { done <- Serve(ctx, server, root, "test", cfg) }()
			defer func() {
				cancel()
				select {
				case err := <-done:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(time.Second):
					t.Error("workers did not stop")
				}
			}()
			// Hold an authenticated upload open. Other clients must transfer files
			// while this client has not even supplied START.
			for i := 0; i < MaxClients+1; i++ {
				if _, err := List(ctx, listen(t), server.LocalAddr(), "wrong", cfg); err == nil || !strings.Contains(err.Error(), "senha incorreta") {
					t.Fatalf("authentication: %v", err)
				}
			}
			stalled := listen(t)
			if _, err := call(ctx, stalled, server.LocalAddr(), Request{Operation: "upload", Name: "stalled", Password: "test"}, cfg); err != nil {
				t.Fatal(err)
			}
			var wg sync.WaitGroup
			errs := make(chan error, 3)
			for i := 0; i < 3; i++ {
				client := listen(t)
				dir := t.TempDir()
				name := fmt.Sprintf("file%d", i)
				content := fmt.Sprintf("payload for client %d", i)
				path := filepath.Join(dir, "source")
				if err := os.WriteFile(path, []byte(content), 0600); err != nil {
					t.Fatal(err)
				}
				wg.Add(1)
				go func() {
					defer wg.Done()
					_, err := Upload(ctx, client, server.LocalAddr(), "test", path, name, cfg)
					if err == nil {
						var b []byte
						b, err = os.ReadFile(filepath.Join(root, name))
						if err == nil && string(b) != content {
							err = fmt.Errorf("cross-client content mismatch")
						}
					}
					// Wait until this endpoint finishes its bounded FINISH linger.
					for err == nil {
						_, readyErr := List(ctx, client, server.LocalAddr(), "test", cfg)
						if readyErr == nil {
							break
						}
						if !strings.Contains(readyErr.Error(), "ocupado") {
							err = readyErr
							break
						}
						time.Sleep(20 * time.Millisecond)
					}
					if err == nil {
						out := filepath.Join(dir, "download")
						_, err = Download(ctx, client, server.LocalAddr(), "test", name, out, cfg)
						if err == nil {
							var b []byte
							b, err = os.ReadFile(out)
							if err == nil && string(b) != content {
								err = fmt.Errorf("download content mismatch")
							}
						}
					}
					errs <- err
				}()
			}
			wg.Wait()
			close(errs)
			for err := range errs {
				if err != nil {
					t.Fatal(err)
				}
			}
			// Fill the remaining slots with authenticated idle clients and check cap.
			for i := 4; i < MaxClients; i++ {
				if _, err := List(ctx, listen(t), server.LocalAddr(), "test", cfg); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := List(ctx, listen(t), server.LocalAddr(), "test", cfg); err == nil {
				t.Fatal("client limit not enforced")
			}
		})
	}
}

func TestPeerConnectionExpiry(t *testing.T) {
	p := &peerConn{closed: make(chan struct{}), packets: make(chan []byte, 1), last: time.Now().Add(-time.Second), lease: time.Millisecond}
	if _, _, err := p.ReadFrom(make([]byte, 1)); err != net.ErrClosed {
		t.Fatalf("idle endpoint retained: %v", err)
	}
	p.Close()
	p.Close()
}
