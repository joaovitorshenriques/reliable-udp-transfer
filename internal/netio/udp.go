package netio

import (
	"fmt"
	"net"
)

// Buffer is requested from the OS; the effective size is platform-dependent.
// Zero preserves the OS defaults for controlled comparisons.
func Listen(address string, buffer int) (net.PacketConn, error) {
	if buffer < 0 || buffer > 64*1024*1024 {
		return nil, fmt.Errorf("udp-buffer must be between 0 and 67108864 bytes")
	}
	c, e := net.ListenPacket("udp", address)
	if e != nil {
		return nil, e
	}
	if buffer > 0 {
		u := c.(*net.UDPConn)
		if e = u.SetReadBuffer(buffer); e != nil {
			c.Close()
			return nil, e
		}
		if e = u.SetWriteBuffer(buffer); e != nil {
			c.Close()
			return nil, e
		}
	}
	return c, nil
}
