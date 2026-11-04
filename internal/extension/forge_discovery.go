package extensionpkg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	c "github.com/compozy/compozy/internal/extension/contract"
	p "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/subprocess"
)

// forgeDiscoveryTimeout bounds one capability lookup plus provider call: a live subprocess can stall.
const forgeDiscoveryTimeout = 15 * time.Second

func (m *Manager) ForgeSearchPRs(ctx context.Context, request c.ForgePRSearchRequest) (c.ForgePRSearchResponse, error) {
	var response c.ForgePRSearchResponse
	err := m.forgeDiscoveryCall(ctx, request.RemoteURLs, p.ExtensionServiceMethodForgePRSearch, request, &response)
	return response, err
}
func (m *Manager) ForgeReadPR(ctx context.Context, request c.ForgePRReadRequest) (c.ForgePRCandidate, error) {
	var response c.ForgePRCandidate
	if request.Number < 1 {
		return response, errors.New("extension: positive pull request number required")
	}
	err := m.forgeDiscoveryCall(ctx, request.RemoteURLs, p.ExtensionServiceMethodForgePRRead, request, &response)
	return response, err
}

func (m *Manager) forgeDiscoveryCall(
	ctx context.Context,
	remotes []string,
	method p.ExtensionServiceMethod,
	request, response any,
) error {
	ctx, cancel := context.WithTimeout(ctx, forgeDiscoveryTimeout)
	defer cancel()
	capabilities, err := m.ForgeCapabilities(ctx, remotes)
	if err != nil {
		return err
	}
	if err := requireAvailableForgeProvider(capabilities); err != nil {
		return err
	}
	if until, limited := m.forgeQuota.limitedUntil(capabilities.Winner, m.now()); limited {
		return forgeRateLimitedError(until)
	}
	if !capabilities.SupportsPRDiscovery {
		return errors.New("extension: pull request discovery unsupported")
	}
	process, name, err := m.extensionServiceProcess(ctx, capabilities.Winner, method)
	if err != nil {
		return err
	}
	if err := process.Call(ctx, string(method), request, response); err != nil {
		if limited, retryAfter := forgeRPCRateLimit(err); limited {
			return forgeRateLimitedError(m.forgeQuota.limit(name, retryAfter, m.now()))
		}
		return fmt.Errorf("extension: forge discovery via %q: %w", name, err)
	}
	return nil
}

// forgeRPCRateLimit reads a provider's JSON-RPC rate-limit refusal: error data
// {"code":"rate_limited","retry_after":"<RFC 3339>"}, with retry_after optional.
func forgeRPCRateLimit(err error) (bool, *time.Time) {
	rpc, ok := errors.AsType[*subprocess.RPCError](err)
	if !ok || len(rpc.Data) == 0 {
		return false, nil
	}
	var data struct {
		Code       string     `json:"code"`
		RetryAfter *time.Time `json:"retry_after"`
	}
	if json.Unmarshal(rpc.Data, &data) != nil || data.Code != c.ForgeCauseRateLimited {
		return false, nil
	}
	return true, data.RetryAfter
}
