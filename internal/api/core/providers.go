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

	"github.com/compozy/agh/internal/api/contract"
	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/providerauth"
	authproviders "github.com/compozy/agh/internal/providers"
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
	if provider.EffectiveAuthMode() == aghconfig.ProviderAuthModeNone {
		classification := authproviders.Classification{
			State:   authproviders.ProviderAuthStateNone,
			Message: authproviders.ProviderAuthNoAuthRequiredMessage,
		}
		c.JSON(http.StatusOK, contract.ProviderAuthProbeResponse{
			Provider:   providerName,
			AuthStatus: providerAuthStatusPayload(provider, classification, h.nowUTC()),
		})
		return
	}
	statusCommand := strings.TrimSpace(provider.AuthStatusCmd)
	if statusCommand == "" {
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
		return
	}
	env, err := h.providerProbeEnv(providerName, provider)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	result, err := h.ProviderAuthRunner(c.Request.Context(), authproviders.ProviderAuthCommandSpec{
		Command: statusCommand,
		Env:     env.CommandEnv,
		Timeout: authproviders.DefaultProviderAuthCommandTimeout,
		NoTTY:   true,
	})
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	classification := authproviders.ClassifyProbeResult(provider, authproviders.ProbeOutcome{
		ExitCode: result.ExitCode,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}, &env)
	c.JSON(http.StatusOK, contract.ProviderAuthProbeResponse{
		Provider:   providerName,
		AuthStatus: providerAuthStatusPayload(provider, classification, h.nowUTC()),
		Probe: &contract.ProviderAuthProbeResult{
			ExitCode:   result.ExitCode,
			Stdout:     diagnostics.RedactAndBound(result.Stdout, maxDiagnosticPayloadBytes),
			Stderr:     diagnostics.RedactAndBound(result.Stderr, maxDiagnosticPayloadBytes),
			DurationMs: result.DurationMs,
		},
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
	provider aghconfig.ProviderConfig,
) (contract.ProviderSummaryPayload, error) {
	env, err := h.providerProbeEnv(providerName, provider)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	classification, err := authproviders.ClassifyDeclared(ctx, provider, &env)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	return contract.ProviderSummaryPayload{
		Name:        providerName,
		DisplayName: strings.TrimSpace(provider.DisplayName),
		Default:     aghconfig.CanonicalProviderName(h.Config.Defaults.Provider) == providerName,
		AuthStatus:  providerAuthStatusPayload(provider, classification, timeZero()),
	}, nil
}

func (h *BaseHandlers) providerProbeEnv(
	providerName string,
	provider aghconfig.ProviderConfig,
) (authproviders.ProbeEnv, error) {
	commandEnv, err := providerauth.CommandEnv(h.HomePaths, providerName, provider, os.Environ())
	if err != nil {
		return authproviders.ProbeEnv{}, err
	}
	return authproviders.ProbeEnv{
		ProviderName: providerName,
		HomePaths:    h.HomePaths,
		LookupEnv:    os.LookupEnv,
		Vault:        h.Vault,
		CommandEnv:   commandEnv,
		RunCommand:   h.ProviderAuthRunner,
	}, nil
}

func (h *BaseHandlers) resolveProvider(
	providerRef string,
) (string, aghconfig.ProviderConfig, error) {
	providerName := aghconfig.CanonicalProviderName(providerRef)
	if providerName == "" {
		return "", aghconfig.ProviderConfig{}, errProviderNotFound
	}
	if !providerExists(&h.Config, providerName) {
		return "", aghconfig.ProviderConfig{}, fmt.Errorf("%w: %q", errProviderNotFound, providerName)
	}
	provider, err := h.Config.ResolveProvider(providerName)
	if err != nil {
		return "", aghconfig.ProviderConfig{}, err
	}
	return providerName, provider, nil
}

func providerExists(cfg *aghconfig.Config, providerName string) bool {
	if _, ok := aghconfig.BuiltinProviders()[providerName]; ok {
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

func providerInventoryNames(cfg *aghconfig.Config) []string {
	seen := make(map[string]struct{})
	for name := range aghconfig.BuiltinProviders() {
		seen[name] = struct{}{}
	}
	if cfg != nil {
		for name := range cfg.Providers {
			canonical := aghconfig.CanonicalProviderName(name)
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
	provider aghconfig.ProviderConfig,
	classification authproviders.Classification,
	lastProbeAt time.Time,
) contract.ProviderAuthStatusPayload {
	return contract.ProviderAuthStatusPayload{
		Mode:        string(provider.EffectiveAuthMode()),
		EnvPolicy:   string(provider.EffectiveEnvPolicy()),
		HomePolicy:  string(provider.EffectiveHomePolicy()),
		State:       string(classification.State),
		Code:        strings.TrimSpace(classification.Code),
		Message:     diagnostics.Redact(strings.TrimSpace(classification.Message)),
		StatusCmd:   strings.TrimSpace(provider.AuthStatusCmd),
		LoginCmd:    strings.TrimSpace(provider.AuthLoginCmd),
		LastProbeAt: optionalTime(lastProbeAt),
	}
}

func timeZero() time.Time {
	return time.Time{}
}
