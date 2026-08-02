package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/providerauth"
	authproviders "github.com/compozy/compozy/internal/providers"
	"github.com/gin-gonic/gin"
)

var (
	errProviderAuthStatusCommandRequired = errors.New("provider auth_status_command is required for remote probe")
	errProviderNotFound                  = errors.New("provider not found")
)

// ListProviders returns the canonical provider inventory and declared auth readiness.
func (h *BaseHandlers) ListProviders(c *gin.Context) {
	response, err := h.providerListResponse(c.Request.Context())
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	c.JSON(http.StatusOK, response)
}

// GetProvider returns one canonical provider summary.
func (h *BaseHandlers) GetProvider(c *gin.Context) {
	providerName, provider, err := h.resolveProvider(c.Param("provider_id"))
	if err != nil {
		h.respondProviderResolutionError(c, err, strings.TrimSpace(c.Param("provider_id")))
		return
	}
	payload, err := h.providerSummaryPayload(c.Request.Context(), providerName, provider)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// ProbeProviderAuth runs a live provider auth status command.
func (h *BaseHandlers) ProbeProviderAuth(c *gin.Context) {
	providerName, provider, err := h.resolveProvider(c.Param("provider_id"))
	if err != nil {
		h.respondProviderResolutionError(c, err, strings.TrimSpace(c.Param("provider_id")))
		return
	}
	if provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNone {
		classification := authproviders.Classification{
			State:   authproviders.ProviderAuthStateNone,
			Message: authproviders.ProviderAuthNoAuthRequiredMessage,
		}
		h.respondProviderAuthStatus(c, providerName, provider, classification, nil)
		return
	}
	statusCommand := strings.TrimSpace(provider.AuthStatusCmd)
	if statusCommand == "" {
		respondProviderAuthStatusCommandRequired(c, providerName)
		return
	}
	env, err := h.providerProbeEnv(providerName, provider)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	declared, err := authproviders.ClassifyDeclared(c.Request.Context(), provider, &env)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	if declared.State == authproviders.ProviderAuthStateMissingCLI {
		h.respondProviderAuthStatus(c, providerName, provider, declared, &env)
		return
	}
	commandSpec, err := authproviders.PrepareAuthStatusCommand(c.Request.Context(), provider, &env)
	if err != nil {
		classification := authproviders.ClassifyError(err)
		h.respondProviderAuthStatus(c, providerName, provider, classification, &env)
		return
	}
	result, err := env.Normalize().RunCommand(c.Request.Context(), commandSpec)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	classification := authproviders.ClassifyProbeResultContext(
		c.Request.Context(),
		provider,
		authproviders.ProbeOutcome{
			ExitCode: result.ExitCode,
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		},
		&env,
	)
	authStatus, err := providerAuthStatusPayload(
		c.Request.Context(), provider, classification, &env, h.nowUTC(),
	)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	c.JSON(http.StatusOK, contract.ProviderAuthProbeResponse{
		Provider:   providerName,
		AuthStatus: authStatus,
		Probe: &contract.ProviderAuthProbeResult{
			ExitCode:   result.ExitCode,
			Stdout:     diagnostics.RedactAndBound(result.Stdout, maxDiagnosticPayloadBytes),
			Stderr:     diagnostics.RedactAndBound(result.Stderr, maxDiagnosticPayloadBytes),
			DurationMs: result.DurationMs,
		},
	})
}

func respondProviderAuthStatusCommandRequired(c *gin.Context, providerName string) {
	classification := authproviders.Classification{
		State:   authproviders.ProviderAuthStateUnknown,
		Code:    contract.CodeProviderClassificationUnknown,
		Message: errProviderAuthStatusCommandRequired.Error(),
		Kind:    authproviders.ProviderFailureUnknown,
		Action:  authproviders.ProviderFailureActionInspect,
	}
	item := authproviders.DiagnosticItem(providerName, classification)
	c.JSON(http.StatusUnprocessableEntity, contract.ErrorPayload{
		Error:      diagnostics.Redact(item.Message),
		Diagnostic: &item,
	})
}

func (h *BaseHandlers) respondProviderAuthStatus(
	c *gin.Context,
	providerName string,
	provider compozyconfig.ProviderConfig,
	classification authproviders.Classification,
	env *authproviders.ProbeEnv,
) {
	authStatus, err := providerAuthStatusPayload(
		c.Request.Context(), provider, classification, env, h.nowUTC(),
	)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	c.JSON(http.StatusOK, contract.ProviderAuthProbeResponse{
		Provider:   providerName,
		AuthStatus: authStatus,
	})
}

func (h *BaseHandlers) providerListResponse(ctx context.Context) (contract.ProviderListResponse, error) {
	providerNames := providerInventoryNames(&h.Config)
	payloads := make([]contract.ProviderSummaryPayload, 0, len(providerNames))
	for _, providerName := range providerNames {
		provider, err := h.Config.ResolveProvider(providerName)
		if err != nil {
			return contract.ProviderListResponse{}, fmt.Errorf("resolve provider %q: %w", providerName, err)
		}
		payload, err := h.providerSummaryPayload(ctx, providerName, provider)
		if err != nil {
			return contract.ProviderListResponse{}, err
		}
		payloads = append(payloads, payload)
	}
	return contract.ProviderListResponse{Providers: payloads}, nil
}

func (h *BaseHandlers) providerSummaryPayload(
	ctx context.Context,
	providerName string,
	provider compozyconfig.ProviderConfig,
) (contract.ProviderSummaryPayload, error) {
	env, err := h.providerProbeEnv(providerName, provider)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	classification, err := authproviders.ClassifyDeclared(ctx, provider, &env)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	authStatus, err := providerAuthStatusPayload(ctx, provider, classification, &env, timeZero())
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	return contract.ProviderSummaryPayload{
		Name:        providerName,
		DisplayName: strings.TrimSpace(provider.DisplayName),
		Default:     compozyconfig.CanonicalProviderName(h.Config.Defaults.Provider) == providerName,
		AuthStatus:  authStatus,
	}, nil
}

func (h *BaseHandlers) providerProbeEnv(
	providerName string,
	provider compozyconfig.ProviderConfig,
) (authproviders.ProbeEnv, error) {
	commandEnv, err := providerauth.CommandEnv(h.HomePaths, providerName, provider, os.Environ())
	if err != nil {
		return authproviders.ProbeEnv{}, err
	}
	return authproviders.ProbeEnv{
		ProviderName:   providerName,
		HomePaths:      h.HomePaths,
		LookupEnv:      os.LookupEnv,
		Vault:          h.Vault,
		CommandEnv:     commandEnv,
		ResolveCommand: h.ProviderAuthCommandResolver,
		RunCommand:     h.ProviderAuthRunner,
	}, nil
}

func (h *BaseHandlers) resolveProvider(
	providerRef string,
) (string, compozyconfig.ProviderConfig, error) {
	providerName := compozyconfig.CanonicalProviderName(providerRef)
	if providerName == "" {
		return "", compozyconfig.ProviderConfig{}, errProviderNotFound
	}
	if !providerExists(&h.Config, providerName) {
		return "", compozyconfig.ProviderConfig{}, fmt.Errorf("%w: %q", errProviderNotFound, providerName)
	}
	provider, err := h.Config.ResolveProvider(providerName)
	if err != nil {
		return "", compozyconfig.ProviderConfig{}, err
	}
	return providerName, provider, nil
}

func providerExists(cfg *compozyconfig.Config, providerName string) bool {
	if _, ok := compozyconfig.BuiltinProviders()[providerName]; ok {
		return true
	}
	if cfg == nil {
		return false
	}
	_, ok := cfg.Providers[providerName]
	return ok
}

func (h *BaseHandlers) respondProviderResolutionError(
	c *gin.Context,
	err error,
	providerName string,
) {
	if errors.Is(err, errProviderNotFound) {
		h.respondProviderNotFound(c, providerName)
		return
	}
	RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
}

func (h *BaseHandlers) respondProviderNotFound(c *gin.Context, providerName string) {
	classification := authproviders.Classification{
		State:   authproviders.ProviderAuthStateUnknown,
		Code:    contract.CodeProviderNotInstalled,
		Message: fmt.Sprintf("Provider %q is not installed or configured.", providerName),
		Kind:    authproviders.ProviderFailureUnknown,
		Action:  authproviders.ProviderFailureActionInspect,
	}
	item := authproviders.DiagnosticItem(providerName, classification)
	c.JSON(http.StatusNotFound, contract.ErrorPayload{
		Error:      diagnostics.Redact(item.Message),
		Diagnostic: &item,
	})
}

func providerInventoryNames(cfg *compozyconfig.Config) []string {
	seen := make(map[string]struct{})
	for name := range compozyconfig.BuiltinProviders() {
		seen[name] = struct{}{}
	}
	if cfg != nil {
		for name := range cfg.Providers {
			canonical := compozyconfig.CanonicalProviderName(name)
			if canonical != "" {
				seen[canonical] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func providerAuthStatusPayload(
	ctx context.Context,
	provider compozyconfig.ProviderConfig,
	classification authproviders.Classification,
	env *authproviders.ProbeEnv,
	lastProbeAt time.Time,
) (contract.ProviderAuthStatusPayload, error) {
	var nativeCLI *providerauth.NativeCLIStatus
	if provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI {
		var err error
		nativeCLI, err = authproviders.NativeCLIStatus(ctx, provider, env)
		if err != nil {
			return contract.ProviderAuthStatusPayload{}, err
		}
	}
	return contract.ProviderAuthStatusPayload{
		Mode:       string(provider.EffectiveAuthMode()),
		EnvPolicy:  string(provider.EffectiveEnvPolicy()),
		HomePolicy: string(provider.EffectiveHomePolicy()),
		State:      string(classification.State),
		Code:       strings.TrimSpace(classification.Code),
		Message:    diagnostics.Redact(strings.TrimSpace(classification.Message)),
		StatusCmd:  strings.TrimSpace(provider.AuthStatusCmd),
		Login: providerLoginDescriptorPayload(
			authproviders.DescribeProviderLogin(provider, nativeCLI, classification),
		),
		LastProbeAt: optionalTime(lastProbeAt),
	}, nil
}

func timeZero() time.Time {
	return time.Time{}
}
