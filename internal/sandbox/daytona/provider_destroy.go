package daytona

import (
	"context"
	"errors"
	"fmt"

	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/sandbox"
)

func (p *daytonaProvider) Destroy(ctx context.Context, state sandbox.SessionState) error {
	if ctx == nil {
		return errors.New("sandbox/daytona: destroy context is required")
	}
	providerState, err := decodeProviderState(state.ProviderState)
	if err != nil {
		return err
	}
	sandboxID := strings.TrimSpace(firstNonEmpty(state.InstanceID, providerState.SandboxID))
	if sandboxID == "" {
		return errors.New("sandbox/daytona: destroy missing sandbox id")
	}
	client, err := p.newClient(clientConfig{APIURL: providerState.APIURL})
	if err != nil {
		return err
	}
	instance, err := p.getSandboxForDestroy(ctx, client, sandboxID)
	if err != nil {
		return err
	}
	switch providerState.Persistence {
	case sandbox.PersistenceArchive:
		return p.withSDKTimeout(ctx, func(ctx context.Context) error {
			return instance.Archive(ctx)
		})
	case sandbox.PersistenceReuse:
		return nil
	default:
		return p.withSDKTimeout(ctx, func(ctx context.Context) error {
			return instance.Delete(ctx)
		})
	}
}

func (p *daytonaProvider) getSandboxForDestroy(
	ctx context.Context,
	client sandboxClient,
	sandboxID string,
) (daytonaSandbox, error) {
	var instance daytonaSandbox
	err := p.withSDKTimeout(ctx, func(ctx context.Context) error {
		var getErr error
		instance, getErr = client.Get(ctx, sandboxID)
		return getErr
	})
	if err != nil {
		return nil, fmt.Errorf("sandbox/daytona: get sandbox %q for destroy: %w", sandboxID, err)
	}
	return instance, nil
}

func (p *daytonaProvider) validateNetworkPolicy(policy sandbox.NetworkPolicy) error {
	unsupported := policy.AllowOutbound || len(policy.AllowList) > 0 || len(policy.DenyList) > 0
	if !unsupported {
		return nil
	}
	message := "sandbox/daytona: network allow_outbound/allow_list/deny_list " +
		"policies are not enforceable by Daytona alpha provider"
	if policy.Required {
		return errors.New(message)
	}
	p.logger.Warn(message)
	return nil
}

func (p *daytonaProvider) withSDKTimeout(ctx context.Context, fn func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, p.sdkTimeout)
	defer cancel()
	if err := fn(timeoutCtx); err != nil {
		return fmt.Errorf("sandbox/daytona: SDK operation failed: %w", err)
	}
	return nil
}

func aghLabels(req sandbox.PrepareRequest) map[string]string {
	return map[string]string{
		"agh_session_id":        req.SessionID,
		providerAghSandboxIDKey: req.SandboxID,
	}
}

func parseDurationMinutes(raw string) *int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if minutes, err := strconv.Atoi(raw); err == nil {
		return &minutes
	}
	duration, err := time.ParseDuration(raw)
	if err != nil {
		return nil
	}
	minutes := int(duration.Minutes())
	return &minutes
}
