package daemon

import (
	"context"
	"time"

	"github.com/compozy/compozy/internal/diagnostics"
	mcpauth "github.com/compozy/compozy/internal/mcp/auth"
)

type settingsSecretRefResolver interface {
	ResolveRef(context.Context, string) (string, error)
}

func settingsRuntimeFunctions(d *Daemon) (func() time.Time, func() int, func() Info) {
	now := time.Now
	pid := func() int { return 0 }
	info := func() Info { return Info{} }
	if d == nil {
		return now, pid, info
	}
	if d.now != nil {
		now = d.now
	}
	if d.pid != nil {
		pid = d.pid
	}
	return now, pid, d.settingsInfoSnapshot
}

func settingsMCPAuthDependencies(
	state *bootState,
) (mcpauth.TokenStore, mcpauth.RegistrationStore, mcpauth.SecretRefResolver, settingsSecretRefResolver) {
	var tokenStore mcpauth.TokenStore
	if store, ok := state.registry.(mcpauth.TokenStore); ok {
		tokenStore = store
	}
	var registrationStore mcpauth.RegistrationStore
	if store, ok := state.registry.(mcpauth.RegistrationStore); ok {
		registrationStore = store
	}
	if state.providerVault == nil {
		return tokenStore, registrationStore, nil, nil
	}
	secretResolver := func(ctx context.Context, ref string) (string, error) {
		value, err := state.providerVault.ResolveRef(ctx, ref)
		if err != nil {
			return "", err
		}
		diagnostics.RegisterDynamicSecret(value)
		return value, nil
	}
	return tokenStore, registrationStore, secretResolver, state.providerVault
}
