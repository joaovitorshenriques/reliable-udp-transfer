// Package netem applies reproducible Bernoulli loss and scheduled datagram delay.
// It models propagation delay, not bandwidth, jitter or a real network topology.
package netem

import (
	"fmt"
	"math"
	"math/rand"
	"net"
	"sync"
	"time"
)

type Config struct {
	Delay time.Duration `json:"delay_ns"`
	Loss  float64       `json:"loss_probability"`
	Seed  int64         `json:"seed"`
	Queue int           `json:"queue_capacity"`
}
type Stats struct {
	Seen              int64   `json:"datagrams_seen"`
	RandomDrops       int64   `json:"random_drops"`
	QueueDrops        int64   `json:"queue_drops"`
	Forwarded         int64   `json:"forwarded"`
	ForwardedBytes    int64   `json:"forwarded_rc2_bytes"`
	SendErrors        int64   `json:"send_errors"`
	PendingOrCanceled int64   `json:"pending_or_canceled"`
	MeanDelayMS       float64 `json:"mean_forward_delay_ms"`
	MaxDelayMS        float64 `json:"max_forward_delay_ms"`
	ObservedLoss      float64 `json:"observed_random_loss"`
	SpanSeconds       float64 `json:"observation_span_seconds"`
	RC2Mbps           float64 `json:"forwarded_rc2_mbps"`
}
type datagram struct {
	data []byte
	addr net.Addr
	at   time.Time
}
type Conn struct {
	net.PacketConn
	cfg         Config
	mu          sync.Mutex
	rng         *rand.Rand
	stats       Stats
	first, last time.Time
	queue       chan datagram
	stop, done  chan struct{}
	closed      bool
}

// Wrap leaves reads unchanged. Each WriteTo is classified once, then scheduled
// independently of earlier packet delays. The underlying socket remains owned
// by the caller; Close stops scheduling and waits for the worker, not the socket.
func Wrap(c net.PacketConn, cfg Config) (*Conn, error) {
	if c == nil || cfg.Delay < 0 || cfg.Delay > 10*time.Second || math.IsNaN(cfg.Loss) || cfg.Loss < 0 || cfg.Loss > 1 || cfg.Queue < 1 || cfg.Queue > 65536 {
		return nil, fmt.Errorf("invalid emulation configuration")
	}
	n := &Conn{PacketConn: c, cfg: cfg, rng: rand.New(rand.NewSource(cfg.Seed)), queue: make(chan datagram, cfg.Queue), stop: make(chan struct{}), done: make(chan struct{})}
	go n.forward()
	return n, nil
}
func (n *Conn) WriteTo(b []byte, a net.Addr) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.closed {
		return 0, net.ErrClosed
	}
	now := time.Now()
	if n.first.IsZero() {
		n.first = now
	}
	n.last = now
	n.stats.Seen++
	if n.rng.Float64() < n.cfg.Loss {
		n.stats.RandomDrops++
		return len(b), nil
	}
	select {
	case n.queue <- datagram{append([]byte(nil), b...), a, now}:
	default:
		n.stats.QueueDrops++
	}
	return len(b), nil
}
func (n *Conn) forward() {
	defer close(n.done)
	for {
		select {
		case <-n.stop:
			return
		case p := <-n.queue:
			timer := time.NewTimer(time.Until(p.at.Add(n.cfg.Delay)))
			select {
			case <-n.stop:
				timer.Stop()
				return
			case <-timer.C:
			}
			size, err := n.PacketConn.WriteTo(p.data, p.addr)
			now := time.Now()
			delay := float64(now.Sub(p.at)) / float64(time.Millisecond)
			n.mu.Lock()
			n.last = now
			if err != nil || size != len(p.data) {
				n.stats.SendErrors++
			} else {
				n.stats.Forwarded++
				n.stats.ForwardedBytes += int64(size)
				n.stats.MeanDelayMS += (delay - n.stats.MeanDelayMS) / float64(n.stats.Forwarded)
				n.stats.MaxDelayMS = max(n.stats.MaxDelayMS, delay)
			}
			n.mu.Unlock()
		}
	}
}
func (n *Conn) Snapshot() Stats {
	n.mu.Lock()
	defer n.mu.Unlock()
	s := n.stats
	s.PendingOrCanceled = s.Seen - s.RandomDrops - s.QueueDrops - s.Forwarded - s.SendErrors
	if s.Seen > 0 {
		s.ObservedLoss = float64(s.RandomDrops) / float64(s.Seen)
		s.SpanSeconds = n.last.Sub(n.first).Seconds()
	}
	if s.SpanSeconds > 0 {
		s.RC2Mbps = float64(s.ForwardedBytes) * 8 / s.SpanSeconds / 1e6
	}
	return s
}
func (n *Conn) Close() error {
	n.mu.Lock()
	if !n.closed {
		n.closed = true
		close(n.stop)
	}
	n.mu.Unlock()
	<-n.done
	return nil
}
