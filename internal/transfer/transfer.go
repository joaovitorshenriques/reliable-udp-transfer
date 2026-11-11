// Package transfer implements a single-file Selective Repeat transfer.
// One goroutine owns connection state. This deliberately avoids shared timer maps.
package transfer

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"trabalho_redes/internal/protocol"
)

type Config struct {
	Window  int
	RTO     time.Duration
	Retries int
	Idle    time.Duration
	MaxSize int64
}

func DefaultConfig() Config {
	return Config{Window: 64, RTO: 500 * time.Millisecond, Retries: 12, Idle: 15 * time.Second, MaxSize: 1 << 30}
}
func (c Config) validate() error {
	if c.Window < 1 || c.Window > 4096 || c.RTO < time.Millisecond || c.RTO > 10*time.Second || c.Retries < 1 || c.Retries > 100 || c.Idle <= 0 || c.MaxSize < 0 || c.MaxSize > 1<<40 {
		return errors.New("invalid transfer configuration")
	}
	return nil
}

type Stats struct {
	Bytes           int64   `json:"bytes"`
	Seconds         float64 `json:"seconds"`
	GoodputMbps     float64 `json:"goodput_mbps"`
	DataSent        int     `json:"data_datagrams_sent"`
	Retransmissions int     `json:"data_retransmissions"`
	ControlRetries  int     `json:"control_retries"`
	Corrupt         int     `json:"corrupt_datagrams_received"`
	Duplicates      int     `json:"duplicate_data_received"`
	Window          int     `json:"window_packets"`
	RTTMillis       float64 `json:"mean_data_rtt_ms"`
	RTTCount        int     `json:"rtt_samples"`
	SHA256          string  `json:"sha256"`
}

func (s *Stats) complete(start time.Time) {
	s.Seconds = time.Since(start).Seconds()
	s.GoodputMbps = float64(s.Bytes) * 8 / s.Seconds / 1e6
}

type metadata struct {
	Size int64  `json:"size"`
	Hash string `json:"sha256"`
}

func emit(c net.PacketConn, a net.Addr, p protocol.Packet) error {
	b, e := p.Marshal()
	if e != nil {
		return e
	}
	_, e = c.WriteTo(b, a)
	return e
}
func read(c net.PacketConn, until time.Time) (protocol.Packet, net.Addr, error) {
	if e := c.SetReadDeadline(until); e != nil {
		return protocol.Packet{}, nil, e
	}
	b := make([]byte, protocol.HeaderSize+protocol.PayloadSize+1)
	n, a, e := c.ReadFrom(b)
	if e != nil {
		if n > 0 && a != nil {
			return protocol.Packet{}, a, e
		}
		return protocol.Packet{}, nil, e
	}
	p, e := protocol.Parse(b[:n])
	return p, a, e
}
func timeout(e error) bool    { var n net.Error; return errors.As(e, &n) && n.Timeout() }
func same(a, b net.Addr) bool { return a != nil && b != nil && a.String() == b.String() }

// exchange retransmits control packets, including FINISH when DONE is lost.
func exchange(ctx context.Context, c net.PacketConn, a net.Addr, p protocol.Packet, want byte, cfg Config, s *Stats) (protocol.Packet, error) {
	for attempt := 0; attempt <= cfg.Retries; attempt++ {
		if e := ctx.Err(); e != nil {
			return protocol.Packet{}, e
		}
		if attempt > 0 {
			s.ControlRetries++
		}
		if e := emit(c, a, p); e != nil {
			return protocol.Packet{}, e
		}
		deadline := time.Now().Add(cfg.RTO)
		for time.Now().Before(deadline) {
			q, peer, e := read(c, deadline)
			if e != nil {
				if timeout(e) {
					break
				}
				if peer != nil {
					s.Corrupt++
					continue
				}
				return protocol.Packet{}, e
			}
			if !same(peer, a) || q.Session != p.Session {
				continue
			}
			if q.Kind == protocol.Failure {
				return protocol.Packet{}, fmt.Errorf("remote: %s", q.Payload)
			}
			if q.Kind == want && q.Seq == p.Seq {
				return q, nil
			}
		}
	}
	return protocol.Packet{}, errors.New("control retry limit exceeded")
}

type pending struct {
	p       protocol.Packet
	sent    time.Time
	retries int
}

func Send(ctx context.Context, c net.PacketConn, a net.Addr, path string, cfg Config) (s Stats, err error) {
	if err = cfg.validate(); err != nil {
		return
	}
	f, e := os.Open(path)
	if e != nil {
		return s, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return s, e
	}
	if !info.Mode().IsRegular() || info.Size() > cfg.MaxSize {
		return s, errors.New("file is not regular or exceeds size limit")
	}
	h := sha256.New()
	if _, e = io.Copy(h, f); e != nil {
		return s, e
	}
	s.SHA256 = hex.EncodeToString(h.Sum(nil))
	s.Bytes = info.Size()
	var id [8]byte
	if _, e = rand.Read(id[:]); e != nil {
		return s, e
	}
	session := binary.BigEndian.Uint64(id[:])
	meta, _ := json.Marshal(metadata{Size: s.Bytes, Hash: s.SHA256})
	start := time.Now()
	defer s.complete(start)
	ack, e := exchange(ctx, c, a, protocol.Packet{Kind: protocol.Start, Session: session, Window: uint16(cfg.Window), Payload: meta}, protocol.Ack, cfg, &s)
	if e != nil {
		return s, e
	}
	window := min(cfg.Window, int(ack.Window))
	if window < 1 {
		return s, errors.New("invalid negotiated window")
	}
	s.Window = window
	count := uint32((s.Bytes + protocol.PayloadSize - 1) / protocol.PayloadSize)
	base, next := uint32(1), uint32(1)
	inflight := map[uint32]*pending{}
	for base <= count {
		if e = ctx.Err(); e != nil {
			return s, e
		}
		for next <= count && next-base < uint32(window) {
			length := min(int64(protocol.PayloadSize), s.Bytes-int64(next-1)*protocol.PayloadSize)
			data := make([]byte, int(length))
			if _, e = f.ReadAt(data, int64(next-1)*protocol.PayloadSize); e != nil {
				return s, e
			}
			p := protocol.Packet{Kind: protocol.Data, Session: session, Seq: next, Payload: data}
			if e = emit(c, a, p); e != nil {
				return s, e
			}
			s.DataSent++
			inflight[next] = &pending{p: p, sent: time.Now()}
			next++
		}
		deadline := time.Now().Add(cfg.RTO)
		for _, v := range inflight {
			if t := v.sent.Add(cfg.RTO); t.Before(deadline) {
				deadline = t
			}
		}
		q, peer, re := read(c, deadline)
		if re == nil && same(peer, a) && q.Session == session {
			if q.Kind == protocol.Failure {
				return s, fmt.Errorf("remote: %s", q.Payload)
			}
			if q.Kind == protocol.Ack {
				if v, ok := inflight[q.Seq]; ok {
					if v.retries == 0 {
						sample := float64(time.Since(v.sent)) / float64(time.Millisecond)
						s.RTTCount++
						s.RTTMillis += (sample - s.RTTMillis) / float64(s.RTTCount)
					}
					delete(inflight, q.Seq)
					for base < next {
						if _, ok := inflight[base]; ok {
							break
						}
						base++
					}
				}
			}
		} else if re != nil && !timeout(re) {
			if peer != nil {
				s.Corrupt++
			} else {
				return s, re
			}
		}
		for _, v := range inflight {
			if time.Since(v.sent) >= cfg.RTO {
				if v.retries >= cfg.Retries {
					return s, fmt.Errorf("retry limit exceeded for sequence %d", v.p.Seq)
				}
				if e = emit(c, a, v.p); e != nil {
					return s, e
				}
				v.sent = time.Now()
				v.retries++
				s.DataSent++
				s.Retransmissions++
			}
		}
	}
	_, e = exchange(ctx, c, a, protocol.Packet{Kind: protocol.Finish, Session: session, Seq: count + 1}, protocol.Done, cfg, &s)
	return s, e
}

// Receive accepts one transfer. Output is committed only after hash validation.
// Existing output is never overwritten; the caller chooses its local filename.
func Receive(ctx context.Context, c net.PacketConn, out string, cfg Config) (s Stats, err error) {
	if err = cfg.validate(); err != nil {
		return
	}
	if _, e := os.Lstat(out); !os.IsNotExist(e) {
		return s, errors.New("output already exists or is inaccessible")
	}
	start := time.Now()
	var peer net.Addr
	var session uint64
	var m metadata
	var f *os.File
	var tmp string
	defer func() {
		if f != nil {
			f.Close()
			os.Remove(tmp)
		}
	}()
	buffer := map[uint32][]byte{}
	base := uint32(1)
	window := cfg.Window
	count := uint32(0)
	h := sha256.New()
	last := time.Now()
	fail := func(message string) (Stats, error) {
		if peer != nil {
			_ = emit(c, peer, protocol.Packet{Kind: protocol.Failure, Session: session, Payload: []byte(message)})
		}
		return s, errors.New(message)
	}
	for {
		if e := ctx.Err(); e != nil {
			return s, e
		}
		if time.Since(last) > cfg.Idle {
			return s, errors.New("receiver idle timeout")
		}
		p, a, e := read(c, time.Now().Add(min(cfg.RTO, 100*time.Millisecond)))
		if e != nil {
			if timeout(e) {
				continue
			}
			if a != nil {
				s.Corrupt++
				continue
			}
			return s, e
		}
		if peer == nil {
			if p.Kind != protocol.Start || p.Seq != 0 || p.Window == 0 {
				continue
			}
			if json.Unmarshal(p.Payload, &m) != nil || m.Size < 0 || m.Size > cfg.MaxSize || len(m.Hash) != 64 {
				continue
			}
			if _, e = hex.DecodeString(m.Hash); e != nil {
				continue
			}
			peer, session = a, p.Session
			window = min(window, int(p.Window))
			s.Window = window
			count = uint32((m.Size + protocol.PayloadSize - 1) / protocol.PayloadSize)
			f, e = os.CreateTemp(filepath.Dir(out), ".rc2-*.part")
			if e != nil {
				return fail("cannot create output")
			}
			tmp = f.Name()
			start = time.Now()
		}
		if !same(peer, a) || p.Session != session {
			continue
		}
		switch p.Kind {
		case protocol.Start:
			last = time.Now()
			if e = emit(c, peer, protocol.Packet{Kind: protocol.Ack, Session: session, Window: uint16(window)}); e != nil {
				return s, e
			}
		case protocol.Data:
			if p.Seq == 0 || p.Seq > count {
				continue
			}
			expected := min(int64(protocol.PayloadSize), m.Size-int64(p.Seq-1)*protocol.PayloadSize)
			if int64(len(p.Payload)) != expected {
				continue
			}
			if p.Seq >= base+uint32(window) {
				continue
			}
			last = time.Now()
			if p.Seq < base {
				s.Duplicates++
			} else if _, ok := buffer[p.Seq]; ok {
				s.Duplicates++
			} else {
				buffer[p.Seq] = p.Payload
			}
			for {
				data, ok := buffer[base]
				if !ok {
					break
				}
				if _, e = f.Write(data); e != nil {
					return fail("output write failed")
				}
				h.Write(data)
				s.Bytes += int64(len(data))
				delete(buffer, base)
				base++
			}
			if e = emit(c, peer, protocol.Packet{Kind: protocol.Ack, Session: session, Seq: p.Seq, Window: uint16(window)}); e != nil {
				return s, e
			}
		case protocol.Finish:
			if p.Seq != count+1 || base != count+1 {
				continue
			}
			s.SHA256 = hex.EncodeToString(h.Sum(nil))
			if s.SHA256 != m.Hash {
				return fail("SHA-256 mismatch")
			}
			if e = f.Sync(); e != nil {
				return fail("output sync failed")
			}
			if e = f.Close(); e != nil {
				return fail("output close failed")
			}
			// Platform operation publishes without replacing an existing path.
			if e = publish(tmp, out); e != nil {
				_, remoteErr := fail("cannot publish output")
				return s, fmt.Errorf("%w: %v", remoteErr, e)
			}
			s.complete(start)
			done := protocol.Packet{Kind: protocol.Done, Session: session, Seq: p.Seq}
			if e = emit(c, peer, done); e != nil {
				return s, e
			}
			// Linger to answer retransmitted FINISH after a lost final response.
			end := time.Now().Add(time.Duration(cfg.Retries+2) * cfg.RTO)
			for time.Now().Before(end) {
				if ctx.Err() != nil {
					return s, nil
				}
				q, addr, re := read(c, minTime(end, time.Now().Add(100*time.Millisecond)))
				if re == nil && same(addr, peer) && q.Session == session && q.Kind == protocol.Finish && q.Seq == done.Seq {
					_ = emit(c, peer, done)
				}
				if re != nil && addr == nil && !timeout(re) {
					return s, re
				}
			}
			return s, nil
		}
	}
}
func minTime(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}
