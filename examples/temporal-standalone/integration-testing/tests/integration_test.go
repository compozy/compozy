//go:build integration

package tests

import (
	"fmt"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/engine/worker/embedded"
	helpers "github.com/compozy/compozy/test/helpers"
)

func TestStandaloneServerLifecycle(t *testing.T) {
	t.Parallel()

	ctx := helpers.NewTestContext(t)
	basePort := findOpenPortRange(t)

	cfg := &embedded.Config{
		DatabaseFile: ":memory:",
		FrontendPort: basePort,
		BindIP:       "127.0.0.1",
		Namespace:    fmt.Sprintf("integration-%d", time.Now().UnixNano()),
		ClusterName:  "integration-testing",
		EnableUI:     false,
		LogLevel:     "warn",
		StartTimeout: 20 * time.Second,
	}

	srv, err := embedded.NewServer(ctx, cfg)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, srv.Stop(ctx))
	})

	require.NoError(t, srv.Start(ctx))
	require.NotEmpty(t, srv.FrontendAddress())
}

// findOpenPortRange locates a contiguous 4-port window for Temporal standalone services.
// The embedded server requires four consecutive ports (one per service) for CI/integration tests.
func findOpenPortRange(t *testing.T) int {
	t.Helper()
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for attempt := 0; attempt < 20; attempt++ {
		base := 20000 + rng.Intn(20000)
		listeners := make([]net.Listener, 0, 4)
		success := true
		for offset := 0; offset < 4; offset++ {
			ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", base+offset))
			if err != nil {
				success = false
				for _, existing := range listeners {
					_ = existing.Close()
				}
				break
			}
			listeners = append(listeners, ln)
		}
		if success {
			for _, ln := range listeners {
				_ = ln.Close()
			}
			return base
		}
	}
	t.Fatalf("unable to allocate contiguous port range for Temporal standalone services")
	return 0
}
