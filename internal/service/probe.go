package service

import (
	"context"
	"fmt"
	"net"
	"time"
)

// Probe performs one authenticated request without retries, avoiding ambiguous RTTs.
// RTT includes application scheduling/authentication, not just propagation.
func Probe(ctx context.Context, c net.PacketConn, peer net.Addr, password string, timeout time.Duration) (time.Duration, error) {
	if timeout < time.Millisecond || timeout > 10*time.Second {
		return 0, fmt.Errorf("probe timeout must be 1ms..10s")
	}
	started := time.Now()
	_, err := callAttempts(ctx, c, peer, Request{Operation: "probe", Password: password}, timeout, 0)
	return time.Since(started), err
}
