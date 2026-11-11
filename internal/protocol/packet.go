// Package protocol implements the educational RC2 datagram format.
package protocol

import (
	"encoding/binary"
	"errors"
	"hash/crc32"
)

const (
	HeaderSize       = 28
	PayloadSize      = 1172 // 1200 bytes total: avoids fragmentation on common paths.
	Start       byte = 1
	Data        byte = 2
	Ack         byte = 3
	Finish      byte = 4
	Done        byte = 5
	Failure     byte = 6
)

type Packet struct {
	Kind    byte
	Session uint64
	Seq     uint32
	Window  uint16
	Payload []byte
}

func (p Packet) Marshal() ([]byte, error) {
	if p.Kind < Start || p.Kind > Failure || len(p.Payload) > PayloadSize {
		return nil, errors.New("invalid packet")
	}
	b := make([]byte, HeaderSize+len(p.Payload))
	copy(b, "RC2U")
	b[4], b[5] = 1, p.Kind
	binary.BigEndian.PutUint64(b[8:16], p.Session)
	binary.BigEndian.PutUint32(b[16:20], p.Seq)
	binary.BigEndian.PutUint16(b[20:22], p.Window)
	binary.BigEndian.PutUint16(b[22:24], uint16(len(p.Payload)))
	copy(b[28:], p.Payload)
	binary.BigEndian.PutUint32(b[24:28], crc32.ChecksumIEEE(b))
	return b, nil
}

func Parse(b []byte) (Packet, error) {
	if len(b) < HeaderSize || len(b) > HeaderSize+PayloadSize || string(b[:4]) != "RC2U" || b[4] != 1 || b[5] < Start || b[5] > Failure || b[6] != 0 || b[7] != 0 {
		return Packet{}, errors.New("invalid header")
	}
	if int(binary.BigEndian.Uint16(b[22:24])) != len(b)-HeaderSize {
		return Packet{}, errors.New("invalid payload length")
	}
	copyB := append([]byte(nil), b...)
	expected := binary.BigEndian.Uint32(copyB[24:28])
	clear(copyB[24:28])
	if crc32.ChecksumIEEE(copyB) != expected {
		return Packet{}, errors.New("checksum mismatch")
	}
	return Packet{Kind: b[5], Session: binary.BigEndian.Uint64(b[8:16]), Seq: binary.BigEndian.Uint32(b[16:20]), Window: binary.BigEndian.Uint16(b[20:22]), Payload: append([]byte(nil), b[28:]...)}, nil
}
