package server

import (
	"context"
	"net"
	"time"
)

func isHostPortReachable(ctx context.Context, hostPort string, timeout time.Duration) bool {
	d := net.Dialer{}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	conn, err := d.DialContext(cctx, "tcp", hostPort)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}
