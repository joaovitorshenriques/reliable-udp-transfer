package transfer

import "net"

// Counters include RC2 datagram bytes, not UDP/IP/link headers or service requests.
type meteredConn struct {
	net.PacketConn
	sent, received, txBytes, rxBytes int64
}

func (m *meteredConn) WriteTo(b []byte, a net.Addr) (int, error) {
	n, e := m.PacketConn.WriteTo(b, a)
	if e == nil {
		m.sent++
		m.txBytes += int64(n)
	}
	return n, e
}
func (m *meteredConn) ReadFrom(b []byte) (int, net.Addr, error) {
	n, a, e := m.PacketConn.ReadFrom(b)
	if e == nil {
		m.received++
		m.rxBytes += int64(n)
	}
	return n, a, e
}
func (m *meteredConn) finish(s *Stats) {
	s.DatagramsSent = m.sent
	s.DatagramsReceived = m.received
	s.TransportBytesSent = m.txBytes
	s.TransportBytesReceived = m.rxBytes
}
