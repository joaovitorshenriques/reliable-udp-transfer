package transfer

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"trabalho_redes/internal/protocol"
)

// Faults act on real UDP sockets. All mutations are deterministic and bounded.
type faulty struct {
	net.PacketConn
	mu       sync.Mutex
	seen     map[string]int
	receiver bool
	held     []byte
}

func (f *faulty) WriteTo(b []byte, a net.Addr) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p, e := protocol.Parse(b)
	if e != nil {
		return f.PacketConn.WriteTo(b, a)
	}
	key := string([]byte{p.Kind}) + string(rune(p.Seq))
	f.seen[key]++
	first := f.seen[key] == 1
	if first && ((f.receiver && (p.Kind == protocol.Done || (p.Kind == protocol.Ack && p.Seq%7 == 0))) || (!f.receiver && (p.Kind == protocol.Start || (p.Kind == protocol.Data && p.Seq%11 == 0)))) {
		return len(b), nil
	}
	if !f.receiver && p.Kind == protocol.Data && first {
		if p.Seq == 1 {
			f.held = append([]byte(nil), b...)
			return len(b), nil
		}
		if p.Seq == 3 {
			bad := append([]byte(nil), b...)
			bad[len(bad)-1] ^= 1
			_, e = f.PacketConn.WriteTo(bad, a)
			return len(b), e
		}
		if p.Seq%5 == 0 {
			if _, e = f.PacketConn.WriteTo(b, a); e != nil {
				return 0, e
			}
		}
	}
	n, e := f.PacketConn.WriteTo(b, a)
	if len(f.held) > 0 && p.Kind == protocol.Data && p.Seq == 2 {
		_, e = f.PacketConn.WriteTo(f.held, a)
		f.held = nil
	}
	return n, e
}
func socket(t *testing.T) net.PacketConn {
	t.Helper()
	c, e := net.ListenPacket("udp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { c.Close() })
	return c
}

func TestTransfers(t *testing.T) {
	for _, tc := range []struct {
		name         string
		size, window int
		faults       bool
	}{{"empty", 0, 8, false}, {"partial", 37, 1, false}, {"window", 260123, 32, false}, {"loss_reorder_duplicate_corruption", 260123, 32, true}, {"fixed_timeout", 260123, 32, true}} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			in, out := filepath.Join(dir, "in.bin"), filepath.Join(dir, "out.bin")
			content := make([]byte, tc.size)
			for i := range content {
				content[i] = byte(i*31 + i/7)
			}
			if e := os.WriteFile(in, content, 0600); e != nil {
				t.Fatal(e)
			}
			var send, recv net.PacketConn = socket(t), socket(t)
			if tc.faults {
				send = &faulty{PacketConn: send, seen: map[string]int{}}
				recv = &faulty{PacketConn: recv, seen: map[string]int{}, receiver: true}
			}
			cfg := DefaultConfig()
			cfg.MaxRTO = 500 * time.Millisecond
			cfg.Adaptive = tc.name != "fixed_timeout"
			cfg.Window = tc.window
			cfg.RTO = 25 * time.Millisecond
			cfg.Retries = 10
			cfg.Idle = 2 * time.Second
			ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			defer cancel()
			type result struct {
				s Stats
				e error
			}
			done := make(chan result, 1)
			go func() { s, e := Receive(ctx, recv, out, cfg); done <- result{s, e} }()
			ss, e := Send(ctx, send, recv.LocalAddr(), in, cfg)
			if e != nil {
				t.Fatal(e)
			}
			rr := <-done
			if rr.e != nil {
				t.Fatal(rr.e)
			}
			got, e := os.ReadFile(out)
			if e != nil || !bytes.Equal(got, content) {
				t.Fatalf("content mismatch: %v", e)
			}
			if ss.SHA256 != rr.s.SHA256 || ss.Bytes != int64(tc.size) {
				t.Fatal("metrics/hash mismatch")
			}
			if ss.Session == "" || ss.Session != rr.s.Session || ss.Role != "sender" || rr.s.Role != "receiver" {
				t.Fatal("missing endpoint identity")
			}
			if ss.DatagramsSent < int64(ss.DataSent) || ss.TransportBytesSent < int64(tc.size) || rr.s.DatagramsReceived == 0 {
				t.Fatal("invalid traffic counters")
			}
			if tc.faults && ss.RTTCount >= (tc.size+protocol.PayloadSize-1)/protocol.PayloadSize {
				t.Fatal("retransmitted packets sampled as RTT")
			}
			if tc.faults && (ss.Retransmissions == 0 || ss.ControlRetries == 0 || rr.s.Corrupt == 0 || rr.s.Duplicates == 0) {
				t.Fatalf("faults not exercised: %+v %+v", ss, rr.s)
			}
		})
	}
}

func TestUnavailableReceiver(t *testing.T) {
	c, blackhole := socket(t), socket(t)
	path := filepath.Join(t.TempDir(), "in")
	os.WriteFile(path, []byte("data"), 0600)
	cfg := DefaultConfig()
	cfg.RTO = 5 * time.Millisecond
	cfg.Retries = 1
	if _, e := Send(context.Background(), c, blackhole.LocalAddr(), path, cfg); e == nil {
		t.Fatal("expected retry exhaustion")
	}
}
func TestExistingOutputPreserved(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out")
	os.WriteFile(path, []byte("original"), 0600)
	if _, e := Receive(context.Background(), socket(t), path, DefaultConfig()); e == nil {
		t.Fatal("expected refusal")
	}
	b, _ := os.ReadFile(path)
	if string(b) != "original" {
		t.Fatal("overwritten")
	}
}
func TestBadHashNotPublished(t *testing.T) {
	recv, send := socket(t), socket(t)
	out := filepath.Join(t.TempDir(), "bad")
	cfg := DefaultConfig()
	cfg.Adaptive = false
	cfg.RTO = 10 * time.Millisecond
	cfg.Idle = time.Second
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { _, e := Receive(ctx, recv, out, cfg); done <- e }()
	wrong := sha256.Sum256([]byte("different"))
	meta, _ := json.Marshal(metadata{Size: 0, Hash: hex.EncodeToString(wrong[:])})
	s := Stats{}
	if _, e := exchange(ctx, send, recv.LocalAddr(), protocol.Packet{Kind: protocol.Start, Session: 7, Window: 1, Payload: meta}, protocol.Ack, cfg, &s); e != nil {
		t.Fatal(e)
	}
	if _, e := exchange(ctx, send, recv.LocalAddr(), protocol.Packet{Kind: protocol.Finish, Session: 7, Seq: 1}, protocol.Done, cfg, &s); e == nil {
		t.Fatal("hash mismatch accepted")
	}
	if e := <-done; e == nil {
		t.Fatal("receiver accepted bad hash")
	}
	if _, e := os.Stat(out); !os.IsNotExist(e) {
		t.Fatal("bad file published")
	}
	entries, _ := os.ReadDir(filepath.Dir(out))
	if len(entries) != 0 {
		t.Fatal("temporary file not cleaned")
	}
}
