package transfer

import (
	"context"
	"errors"
	"net"
	"os"
	"testing"
	"time"

	"trabalho_redes/internal/protocol"
)

// Silent connection models an unavailable peer without depending on host UDP.
type silentConn struct {
	net.PacketConn
	deadline time.Time
	entered  chan struct{}
	writes   int
}

func (c *silentConn) SetReadDeadline(d time.Time) error { c.deadline = d; return nil }
func (c *silentConn) ReadFrom([]byte) (int, net.Addr, error) {
	select {
	case c.entered <- struct{}{}:
	default:
	}
	time.Sleep(time.Until(c.deadline))
	return 0, nil, os.ErrDeadlineExceeded
}
func (c *silentConn) WriteTo(b []byte, _ net.Addr) (int, error) { c.writes++; return len(b), nil }

func TestControlCancellationWithLongRTO(t *testing.T) {
	c := &silentConn{entered: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := DefaultConfig()
	cfg.RTO = 5 * time.Second
	done := make(chan error, 1)
	go func() {
		_, err := exchange(ctx, c, &net.UDPAddr{}, protocol.Packet{Kind: protocol.Start}, protocol.Ack, cfg, &Stats{})
		done <- err
	}()
	select {
	case <-c.entered:
	case <-time.After(time.Second):
		t.Fatal("did not enter read")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("error=%v", err)
		}
		if c.writes != 1 {
			t.Fatalf("polling caused retransmission: %d", c.writes)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation waited for the full RTO")
	}
}

func TestControlPollingPreservesRTO(t *testing.T) {
	c := &silentConn{entered: make(chan struct{}, 1)}
	cfg := DefaultConfig()
	cfg.RTO = 250 * time.Millisecond
	cfg.Retries = 1
	start := time.Now()
	s := Stats{}
	_, err := exchange(context.Background(), c, &net.UDPAddr{}, protocol.Packet{Kind: protocol.Start}, protocol.Ack, cfg, &s)
	if err == nil || c.writes != 2 || s.ControlRetries != 1 || time.Since(start) < 2*cfg.RTO {
		t.Fatalf("incorrect retry timing: writes=%d stats=%+v error=%v", c.writes, s, err)
	}
}
