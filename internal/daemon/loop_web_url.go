package daemon

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	looppkg "github.com/compozy/compozy/internal/loop"
)

const loopRunWebScheme = "http"

type loopRunWebEndpoint struct {
	host string
}

func (s *daemonLoopAPIService) resolveLoopRunWebEndpoint(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
) (loopRunWebEndpoint, error) {
	cfg, err := resolveLoopServiceConfig(ctx, s.homePaths, s.workspaceResolver, workspaceID)
	if err != nil {
		return loopRunWebEndpoint{}, fmt.Errorf("daemon: resolve Loop run web URL config: %w", err)
	}
	host := strings.Trim(strings.TrimSpace(cfg.HTTP.Host), "[]")
	if host == "" || cfg.HTTP.Port <= 0 {
		return loopRunWebEndpoint{}, fmt.Errorf(
			"daemon: resolve Loop run web URL: effective HTTP host and port are required",
		)
	}
	return loopRunWebEndpoint{host: net.JoinHostPort(host, strconv.Itoa(cfg.HTTP.Port))}, nil
}

func (e loopRunWebEndpoint) runURL(runID string) string {
	rawRunID := strings.TrimSpace(runID)
	path := fmt.Sprintf(contract.LoopRunWebRoute, rawRunID)
	rawPath := fmt.Sprintf(contract.LoopRunWebRoute, url.PathEscape(rawRunID))
	return (&url.URL{
		Scheme:  loopRunWebScheme,
		Host:    e.host,
		Path:    path,
		RawPath: rawPath,
	}).String()
}
