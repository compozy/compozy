package config

import (
	"errors"
	"fmt"

	"strings"

	"github.com/compozy/agh/internal/sandbox"
	"github.com/compozy/agh/internal/vault"
)

// Validate ensures the loaded configuration is internally consistent.
func (c *Config) Validate() error {
	return c.validateWithEnv(processEnvLookup)
}

func (c *Config) validateWithEnv(lookup envLookup) error {
	if c == nil {
		return errors.New("config is required")
	}
	if err := c.validateCore(); err != nil {
		return err
	}
	if err := c.validateFeatures(lookup); err != nil {
		return err
	}
	if err := c.validateProviders(); err != nil {
		return err
	}
	if err := c.validateSandboxes(); err != nil {
		return err
	}
	return nil
}

func (c *Config) validateCore() error {
	if err := c.Daemon.Validate(); err != nil {
		return err
	}
	if err := c.HTTP.Validate(); err != nil {
		return err
	}
	if err := c.WindowManager.Validate(); err != nil {
		return err
	}
	if err := c.Defaults.Validate(); err != nil {
		return err
	}
	if err := c.Limits.Validate(); err != nil {
		return err
	}
	if err := c.Session.Validate(); err != nil {
		return err
	}
	if err := c.Permissions.Validate(); err != nil {
		return err
	}
	if err := c.Roles.Validate("roles", c); err != nil {
		return err
	}
	for i, server := range c.MCPServers {
		if err := server.Validate(fmt.Sprintf("mcp_servers[%d]", i)); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validateProviders() error {
	for name := range c.Providers {
		if _, err := c.ResolveProvider(name); err != nil {
			return err
		}
	}
	if provider := strings.TrimSpace(c.Defaults.Provider); provider != "" {
		if _, err := c.ResolveProvider(provider); err != nil {
			return err
		}
	}

	return nil
}

func (c *Config) validateSandboxes() error {
	for name, profile := range c.Sandboxes {
		trimmedName := strings.TrimSpace(name)
		if trimmedName == "" {
			return errors.New("sandboxes: profile name is required")
		}
		if err := profile.Validate(fmt.Sprintf("sandboxes.%s", trimmedName)); err != nil {
			return err
		}
	}
	if ref := strings.TrimSpace(c.Defaults.Sandbox); ref != "" {
		if _, err := c.ResolveSandbox(ref); err != nil {
			return fmt.Errorf("defaults.sandbox: %w", err)
		}
	}

	return nil
}

// ResolveSandbox resolves a named sandbox profile into runtime policy.
func (c *Config) ResolveSandbox(ref string) (sandbox.Resolved, error) {
	profileName := strings.TrimSpace(ref)
	if profileName == "" {
		profileName = string(sandbox.BackendLocal)
	}

	profile, ok := c.Sandboxes[profileName]
	if !ok {
		if profileName == string(sandbox.BackendLocal) {
			return defaultLocalSandbox(), nil
		}
		return sandbox.Resolved{}, fmt.Errorf("%w: %q", ErrSandboxProfileNotFound, profileName)
	}

	resolved, err := profile.Resolve(profileName)
	if err != nil {
		return sandbox.Resolved{}, err
	}
	return resolved, nil
}

func defaultLocalSandbox() sandbox.Resolved {
	return sandbox.Resolved{
		Profile:       string(sandbox.BackendLocal),
		Backend:       sandbox.BackendLocal,
		SyncMode:      sandbox.SyncModeNone,
		Persistence:   sandbox.PersistenceTransient,
		DestroyOnStop: false,
	}
}

// Validate ensures the sandbox profile is internally consistent.
func (p SandboxProfile) Validate(path string) error {
	backend := sandbox.Backend(strings.TrimSpace(p.Backend))
	if !backend.Valid() {
		return fmt.Errorf(
			"%s.backend must be one of %q, %q, %q: %q",
			path,
			sandbox.BackendLocal,
			sandbox.BackendDaytona,
			sandbox.BackendE2B,
			p.Backend,
		)
	}

	if syncMode := strings.TrimSpace(p.SyncMode); syncMode != "" {
		mode := sandbox.SyncMode(syncMode)
		if !mode.Valid() {
			return fmt.Errorf(
				"%s.sync_mode must be one of %q, %q, %q: %q",
				path,
				sandbox.SyncModeNone,
				sandbox.SyncModeSessionBidirectional,
				sandbox.SyncModeTurnBidirectional,
				p.SyncMode,
			)
		}
	}

	if persistenceMode := strings.TrimSpace(p.Persistence); persistenceMode != "" {
		mode := sandbox.PersistenceMode(persistenceMode)
		if !mode.Valid() {
			return fmt.Errorf(
				"%s.persistence must be one of %q, %q, %q: %q",
				path,
				sandbox.PersistenceTransient,
				sandbox.PersistenceReuse,
				sandbox.PersistenceArchive,
				p.Persistence,
			)
		}
	}
	if err := vault.ValidateNonSecretEnvMap(path, p.Env); err != nil {
		return err
	}
	if err := vault.ValidateSecretEnvMap(path, "sandbox", p.SecretEnv); err != nil {
		return err
	}

	return nil
}

// Resolve converts one validated config profile into runtime sandbox policy.
func (p SandboxProfile) Resolve(profileName string) (sandbox.Resolved, error) {
	if err := p.Validate("sandbox profile " + profileName); err != nil {
		return sandbox.Resolved{}, err
	}

	backend := sandbox.Backend(strings.TrimSpace(p.Backend))
	syncMode := sandbox.SyncMode(strings.TrimSpace(p.SyncMode))
	if syncMode == "" {
		syncMode = defaultSyncModeForBackend(backend)
	}
	persistence := sandbox.PersistenceMode(strings.TrimSpace(p.Persistence))
	if persistence == "" {
		persistence = sandbox.PersistenceTransient
	}

	resolved := sandbox.Resolved{
		Profile:        strings.TrimSpace(profileName),
		Backend:        backend,
		SyncMode:       syncMode,
		Persistence:    persistence,
		RuntimeRootDir: strings.TrimSpace(p.RuntimeRoot),
		DestroyOnStop:  persistence != sandbox.PersistenceReuse,
		Env:            mergeStringMaps(nil, p.Env),
		SecretEnv:      mergeStringMaps(nil, p.SecretEnv),
		Network: sandbox.NetworkPolicy{
			AllowPublicIngress: p.Network.AllowPublicIngress,
			AllowOutbound:      p.Network.AllowOutbound,
			AllowList:          cloneStrings(p.Network.AllowList),
			DenyList:           cloneStrings(p.Network.DenyList),
			Required:           p.Network.Required,
		},
	}
	if backend == sandbox.BackendDaytona {
		daytona := p.Daytona.Resolve()
		resolved.Daytona = &daytona
	}

	return resolved, nil
}

func defaultSyncModeForBackend(backend sandbox.Backend) sandbox.SyncMode {
	if backend == sandbox.BackendLocal {
		return sandbox.SyncModeNone
	}
	return sandbox.SyncModeSessionBidirectional
}

// Resolve converts Daytona profile inputs into provider startup policy.
func (p DaytonaProfile) Resolve() sandbox.DaytonaConfig {
	resolved := sandbox.DaytonaConfig{
		APIURL:      strings.TrimSpace(p.APIURL),
		Target:      strings.TrimSpace(p.Target),
		Image:       strings.TrimSpace(p.Image),
		Snapshot:    strings.TrimSpace(p.Snapshot),
		Class:       strings.TrimSpace(p.Class),
		AutoStop:    strings.TrimSpace(p.AutoStop),
		AutoArchive: strings.TrimSpace(p.AutoArchive),
	}
	switch {
	case resolved.Snapshot != "":
		resolved.StartupSource = sandbox.DaytonaStartupSourceSnapshot
		resolved.StartupRef = resolved.Snapshot
	case resolved.Image != "":
		resolved.StartupSource = sandbox.DaytonaStartupSourceImage
		resolved.StartupRef = resolved.Image
	}
	return resolved
}

// Validate ensures the default agent setting is present.
func (c DefaultsConfig) Validate() error {
	if strings.TrimSpace(c.Agent) == "" {
		return ValidationError{Path: configDefaultsAgentPath, Message: "is required"}
	}

	return nil
}
