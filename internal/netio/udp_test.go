package netio

import (
	"testing"
	"time"
)

func TestListenBuffers(t *testing.T) {
	for _, size := range []int{0, 4 * 1024 * 1024} {
		c, err := Listen("127.0.0.1:0", size)
		if err != nil {
			t.Fatal(err)
		}
		// Check actual UDP I/O without assuming the OS grants the requested size.
		if err = c.SetDeadline(time.Now().Add(2 * time.Second)); err != nil {
			c.Close()
			t.Fatal(err)
		}
		if _, err = c.WriteTo([]byte("probe"), c.LocalAddr()); err != nil {
			c.Close()
			t.Fatal(err)
		}
		buf := make([]byte, 32)
		n, _, err := c.ReadFrom(buf)
		c.Close()
		if err != nil || string(buf[:n]) != "probe" {
			t.Fatalf("buffer=%d: payload=%q error=%v", size, buf[:n], err)
		}
	}
}

func TestListenRejectsInvalidBuffers(t *testing.T) {
	for _, size := range []int{-1, 64*1024*1024 + 1} {
		c, err := Listen("127.0.0.1:0", size)
		if err == nil {
			c.Close()
			t.Fatalf("accepted buffer %d", size)
		}
	}
}
