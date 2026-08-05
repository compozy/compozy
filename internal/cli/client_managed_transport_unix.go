//go:build !windows

package cli

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
)

const managedAgentTransportLabel = "managed agent transport"

func managedAgentTransportClients() (*http.Client, *http.Client, bool, error) {
	socketPath := strings.TrimSpace(os.Getenv(agentidentity.EnvTransportSocket))
	if socketPath == "" {
		return nil, nil, false, nil
	}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var dialer net.Dialer
			return dialer.DialContext(ctx, "unix", socketPath)
		},
	}
	return &http.Client{Transport: transport, Timeout: defaultUnixSocketClientTimeout},
		&http.Client{Transport: transport}, true, nil
}
