package modelcatalog

import (
	"context"
	"errors"
	"fmt"
	"io"

	"net/http"
	"net/url"

	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/procutil"
	"github.com/compozy/agh/internal/providerenv"
	"github.com/compozy/agh/internal/vault"
)

func defaultEndpointPath(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Path == "" {
		return ""
	}
	if parsed.RawQuery != "" {
		return parsed.Path + "?" + parsed.RawQuery
	}
	return parsed.Path
}

func (s *LiveProviderSource) discoveryEnv(ctx context.Context) ([]string, error) {
	env := append([]string(nil), s.baseEnv...)
	switch s.provider.EffectiveEnvPolicy() {
	case aghconfig.ProviderEnvPolicyIsolated:
		env = procutil.IsolatedDaemonEnv(env)
	default:
		env = procutil.FilteredDaemonEnv(env)
	}
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER", s.providerID)
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_AUTH_MODE", string(s.provider.EffectiveAuthMode()))
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_ENV_POLICY", string(s.provider.EffectiveEnvPolicy()))
	env = providerenv.SetEnvValue(env, "AGH_PROVIDER_HOME_POLICY", string(s.provider.EffectiveHomePolicy()))

	var err error
	env, err = providerenv.ApplyHomePolicy(s.homePaths, s.providerID, s.provider.EffectiveHomePolicy(), env)
	if err != nil {
		return nil, fmt.Errorf("model catalog: apply provider home policy for %q: %w", s.providerID, err)
	}
	if s.provider.EffectiveHarness() == aghconfig.ProviderHarnessPiACP &&
		s.provider.EffectiveAuthMode() == aghconfig.ProviderAuthModeNativeCLI {
		env, err = providerenv.ApplyPiAgentDirPolicy(s.homePaths, s.providerID, s.provider.EffectiveHomePolicy(), env)
		if err != nil {
			return nil, fmt.Errorf("model catalog: apply pi discovery home policy for %q: %w", s.providerID, err)
		}
	}
	if s.provider.EffectiveAuthMode() != aghconfig.ProviderAuthModeBoundSecret {
		return env, nil
	}
	for _, slot := range s.provider.EffectiveCredentialSlots() {
		next, err := s.injectProviderSecret(ctx, env, slot)
		if err != nil {
			return nil, err
		}
		env = next
	}
	return env, nil
}

func (s *LiveProviderSource) injectProviderSecret(
	ctx context.Context,
	env []string,
	slot aghconfig.ProviderCredentialSlot,
) ([]string, error) {
	targetEnv := strings.TrimSpace(slot.TargetEnv)
	secretRef := vault.NormalizeRef(slot.SecretRef)
	if targetEnv == "" || secretRef == "" {
		return env, nil
	}
	value, err := s.secretResolver.ResolveRef(ctx, secretRef)
	if err != nil {
		if !slot.Required && (errors.Is(err, vault.ErrMissingSecret) || errors.Is(err, vault.ErrSecretNotFound)) {
			return env, nil
		}
		return nil, fmt.Errorf("model catalog: resolve provider credential %q for %q: %w", slot.Name, s.providerID, err)
	}
	return providerenv.SetEnvValue(env, targetEnv, value), nil
}

func (s *LiveProviderSource) listHTTP(
	ctx context.Context,
	endpoint string,
	env []string,
	timeout time.Duration,
	now time.Time,
) (rows []ModelRow, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("model catalog: create live discovery request for %q: %w", s.providerID, err)
	}
	for key, value := range s.adapter.headers {
		req.Header.Set(key, value)
	}
	if err := s.applyRequestAuth(req, env); err != nil {
		return nil, err
	}
	client := s.httpClient
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}
	resp, err := client.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf(
				"model catalog: live discovery for %q timed out after %s: %w",
				s.providerID,
				timeout,
				ctx.Err(),
			)
		}
		return nil, fmt.Errorf("model catalog: fetch live models for %q: %w", s.providerID, err)
	}
	defer func() {
		if _, copyErr := io.Copy(io.Discard, resp.Body); copyErr != nil && err == nil {
			err = fmt.Errorf("model catalog: drain live discovery response for %q: %w", s.providerID, copyErr)
		}
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("model catalog: close live discovery response for %q: %w", s.providerID, closeErr)
		}
	}()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("model catalog: live discovery for %q returned HTTP %d", s.providerID, resp.StatusCode)
	}
	payload, err := io.ReadAll(io.LimitReader(resp.Body, maxLiveDiscoveryPayloadSize))
	if err != nil {
		return nil, fmt.Errorf("model catalog: read live discovery response for %q: %w", s.providerID, err)
	}
	rows, err = parseLiveModelPayload(s.providerID, payload, now)
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (s *LiveProviderSource) applyRequestAuth(req *http.Request, env []string) error {
	credential := firstEnvValue(env, s.adapter.credentialEnvKeys...)
	if credential == "" && s.adapter.authRequired {
		return fmt.Errorf(
			"model catalog: provider %q live discovery requires a bound_secret credential",
			s.providerID,
		)
	}
	if credential == "" {
		return nil
	}
	switch s.adapter.authScheme {
	case liveAuthBearer:
		req.Header.Set("Authorization", "Bearer "+credential)
	case liveAuthAnthropic:
		req.Header.Set("x-api-key", credential)
	case liveAuthGemini:
		req.Header.Set("x-goog-api-key", credential)
	case liveAuthNone:
		return nil
	default:
		return fmt.Errorf("model catalog: unsupported live discovery auth scheme %q", s.adapter.authScheme)
	}
	return nil
}
