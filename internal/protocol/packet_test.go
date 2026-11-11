package protocol

import "testing"

func TestPacketValidation(t *testing.T) {
	p := Packet{Kind: Data, Session: 123, Seq: 9, Window: 64, Payload: []byte("arquivo")}
	b, e := p.Marshal()
	if e != nil {
		t.Fatal(e)
	}
	q, e := Parse(b)
	if e != nil || q.Seq != p.Seq || q.Session != p.Session || string(q.Payload) != "arquivo" {
		t.Fatalf("round trip: %+v %v", q, e)
	}
	for i := range b {
		bad := append([]byte(nil), b...)
		bad[i] ^= 1
		if _, e := Parse(bad); e == nil {
			t.Fatalf("corruption accepted at byte %d", i)
		}
	}
	for i := 0; i < len(b); i++ {
		if _, e := Parse(b[:i]); e == nil {
			t.Fatalf("truncation accepted: %d", i)
		}
	}
	if _, e := Parse(append(b, 0)); e == nil {
		t.Fatal("trailing data accepted")
	}
}
func FuzzParse(f *testing.F) {
	b, _ := (Packet{Kind: Data, Payload: []byte("test")}).Marshal()
	f.Add(b)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) { _, _ = Parse(b) })
}
