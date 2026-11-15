package netem

import (
	"net"
	"testing"
	"time"
)

type capture struct {
	net.PacketConn
	sent chan time.Time
}

func (c *capture) WriteTo(b []byte, _ net.Addr) (int, error) {
	c.sent <- time.Now()
	return len(b), nil
}
func TestScheduledBurst(t *testing.T) {
	c := &capture{sent: make(chan time.Time, 10)}
	n, err := Wrap(c, Config{Delay: 100 * time.Millisecond, Queue: 16, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer n.Close()
	start := time.Now()
	for i := 0; i < 10; i++ {
		if _, err = n.WriteTo([]byte("payload"), nil); err != nil {
			t.Fatal(err)
		}
	}
	var first, last time.Time
	for i := 0; i < 10; i++ {
		select {
		case at := <-c.sent:
			if i == 0 {
				first = at
			}
			last = at
		case <-time.After(2 * time.Second):
			t.Fatal("forward timeout")
		}
	}
	n.Close()
	if first.Sub(start) < 100*time.Millisecond {
		t.Fatal("forwarded before propagation delay")
	}
	if last.Sub(first) > 500*time.Millisecond {
		t.Fatal("burst serialized by delay")
	}
	s := n.Snapshot()
	if s.Seen != 10 || s.Forwarded != 10 || s.ForwardedBytes != 70 || s.PendingOrCanceled != 0 {
		t.Fatalf("accounting: %+v", s)
	}
}
func TestLossAndQueueAccounting(t *testing.T) {
	for _, loss := range []float64{0, 0.5, 1} {
		var previous Stats
		for run := 0; run < 2; run++ {
			n, err := Wrap(&capture{sent: make(chan time.Time, 100)}, Config{Delay: time.Second, Queue: 2, Loss: loss, Seed: 42})
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 100; i++ {
				n.WriteTo([]byte("x"), nil)
			}
			n.Close()
			s := n.Snapshot()
			if s.Seen != 100 || s.Forwarded != 0 || s.Seen != s.RandomDrops+s.QueueDrops+s.PendingOrCanceled {
				t.Fatalf("bad accounting: %+v", s)
			}
			if loss == 1 && s.RandomDrops != 100 {
				t.Fatal("100% loss failed")
			}
			if loss == 0 && s.RandomDrops != 0 {
				t.Fatal("0% loss failed")
			}
			if run == 1 && s.RandomDrops != previous.RandomDrops {
				t.Fatal("loss seed is not reproducible")
			}
			previous = s
			if _, err = n.WriteTo([]byte("x"), nil); err != net.ErrClosed {
				t.Fatal("write after close accepted")
			}
		}
	}
}
func TestInvalidConfig(t *testing.T) {
	for _, cfg := range []Config{{Queue: 0}, {Queue: 1, Delay: -1}, {Queue: 1, Loss: -0.1}, {Queue: 1, Loss: 1.1}} {
		if n, err := Wrap(&capture{}, cfg); err == nil {
			n.Close()
			t.Fatalf("accepted %+v", cfg)
		}
	}
}
