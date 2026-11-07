//go:build !windows

package cli

import (
	"context"
	"net"
)

func dialAppControl(ctx context.Context, path string) (net.Conn, error) {
	return (&net.Dialer{}).DialContext(ctx, "unix", path)
}
