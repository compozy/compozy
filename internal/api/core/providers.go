package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
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
	cfg, err := h.activeConfig(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	providerName, provider, err := resolveProvider(&cfg, c.Param("provider_id"))
	if err != nil {
		h.respondProviderResolutionError(c, err, strings.TrimSpace(c.Param("provider_id")))
		return
	}
	payload, err := h.providerSummaryPayload(c.Request.Context(), &cfg, providerName, provider)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	c.JSON(http.StatusOK, payload)
}

// ProbeProviderAuth runs a live provider auth status command.
func (h *BaseHandlers) ProbeProviderAuth(c *gin.Context) {
	cfg, err := h.activeConfig(c.Request.Context())
	if err != nil {
		h.respondError(c, http.StatusInternalServerError, err)
		return
	}
	providerName, provider, err := resolveProvider(&cfg, c.Param("provider_id"))
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
		h.probeProviderAuthWithoutStatusCommand(c, providerName, provider)
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
	probe, err := authproviders.RunAuthStatusProbe(c.Request.Context(), provider, &env, h.nowUTC)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	if !probe.Ran {
		h.respondProviderAuthStatus(c, providerName, provider, probe.Classification, &env)
		return
	}
	h.observeProviderAuthProbe(
		c.Request.Context(),
		providerName,
		provider,
		env.CommandEnv,
		probe.Classification,
		probe.StartedAt,
	)
	authStatus, err := providerAuthStatusPayload(
		c.Request.Context(), provider, probe.Classification, &env, h.nowUTC(),
	)
	if err != nil {
		RespondError(c, http.StatusInternalServerError, err, h.MaskInternalErrors)
		return
	}
	c.JSON(http.StatusOK, contract.ProviderAuthProbeResponse{
		Provider:   providerName,
		AuthStatus: authStatus,
		Probe: &contract.ProviderAuthProbeResult{
			ExitCode:   probe.Result.ExitCode,
			Stdout:     authproviders.RedactAuthProbeOutput(probe.Result.Stdout, maxDiagnosticPayloadBytes),
			Stderr:     authproviders.RedactAuthProbeOutput(probe.Result.Stderr, maxDiagnosticPayloadBytes),
			DurationMs: probe.Result.DurationMs,
		},
	})
}

func (h *BaseHandlers) probeProviderAuthWithoutStatusCommand(
	c *gin.Context, providerName string, provider compozyconfig.ProviderConfig,
) {
	if authproviders.UsesACPProbe(provider) &&
		provider.EffectiveAuthMode() == compozyconfig.ProviderAuthModeNativeCLI {
		h.probeProviderACPAuth(c, providerName, provider)
		return
	}
	respondProviderAuthStatusCommandRequired(c, providerName)
}

// probeProviderACPAuth runs one non-interactive disposable ACP probe for
// providers without a native status command. It never sends authenticate and
// omits the native command-result probe object. A successful probe clears a
// cached startup failure through the existing invalidation hook; any other
// outcome leaves the cache untouched.
func (h *BaseHandlers) probeProviderACPAuth(
	c *gin.Context,
	providerName string,
	provider compozyconfig.ProviderConfig,
) {
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
	probe := h.ProviderACPProbe
	if probe == nil {
		probe = acp.ProbeAuthentication
	}
	startedAt := h.nowUTC()
	probeErr := probe(c.Request.Context(), acp.SessionInspectionRequest{
		AgentName: providerName,
		Command:   strings.TrimSpace(provider.Command),
		Cwd:       env.CommandDir,
		Env:       append([]string(nil), env.CommandEnv...),
	})
	if ctxErr := c.Request.Context().Err(); ctxErr != nil {
		if probeErr == nil {
			probeErr = ctxErr
		}
		RespondError(c, http.StatusInternalServerError, probeErr, h.MaskInternalErrors)
		return
	}
	classification := acp.ClassifyProbeError(providerName, probeErr)
	// A probe that hit its own deadline or a cancellation says nothing reliable about sign-in, even
	// when an auth-required error is joined to it.
	if !errors.Is(probeErr, context.DeadlineExceeded) && !errors.Is(probeErr, context.Canceled) {
		h.observeProviderAuthProbe(
			c.Request.Context(),
			providerName,
			provider,
			env.CommandEnv,
			classification,
			startedAt,
		)
	}
	h.respondProviderAuthStatus(c, providerName, provider, classification, &env)
}

// observeProviderAuthProbe keeps a completed live probe's verdict for later provider reads and
// clears cached startup failures after a successful sign-in. Canceled probes change nothing.
func (h *BaseHandlers) observeProviderAuthProbe(
	ctx context.Context,
	providerName string,
	provider compozyconfig.ProviderConfig,
	env []string,
	classification authproviders.Classification,
	startedAt time.Time,
) {
	if ctx.Err() != nil {
		return
	}
	h.ProviderAuthVerdicts.Record(providerName, provider, env, classification, startedAt)
	if classification.State == authproviders.ProviderAuthStateAuthenticated && h.OnProviderAuthSuccess != nil {
		h.OnProviderAuthSuccess()
	}
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
	cfg, err := h.activeConfig(ctx)
	if err != nil {
		return contract.ProviderListResponse{}, err
	}
	providerNames := compozyconfig.ProviderInventoryNames(&cfg)
	payloads := make([]contract.ProviderSummaryPayload, 0, len(providerNames))
	for _, providerName := range providerNames {
		provider, err := cfg.ResolveProvider(providerName)
		if err != nil {
			return contract.ProviderListResponse{}, fmt.Errorf("resolve provider %q: %w", providerName, err)
		}
		payload, err := h.providerSummaryPayload(ctx, &cfg, providerName, provider)
		if err != nil {
			return contract.ProviderListResponse{}, err
		}
		payloads = append(payloads, payload)
	}
	return contract.ProviderListResponse{Providers: payloads}, nil
}

func (h *BaseHandlers) providerSummaryPayload(
	ctx context.Context,
	cfg *compozyconfig.Config,
	providerName string,
	provider compozyconfig.ProviderConfig,
) (contract.ProviderSummaryPayload, error) {
	env, err := h.providerProbeEnv(providerName, provider)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	declared, err := authproviders.ClassifyDeclared(ctx, provider, &env)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	// Reads never run a status command; they report the last live verdict, if any.
	classification, checkedAt := h.ProviderAuthVerdicts.Apply(providerName, provider, env.CommandEnv, declared)
	authStatus, err := providerAuthStatusPayload(ctx, provider, classification, &env, checkedAt)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	sessionEnv, err := authproviders.SessionEnvStatuses(ctx, provider.SessionEnv, &env)
	if err != nil {
		return contract.ProviderSummaryPayload{}, err
	}
	return contract.ProviderSummaryPayload{
		ReviewerMode:     provider.ReviewerMode,
		SessionEnvStatus: sessionEnvStatusPayloads(sessionEnv),
		Name:             providerName,
		DisplayName:      strings.TrimSpace(provider.DisplayName),
		RuntimeStrategy:  providerRuntimeStrategy(providerName, provider.RuntimeProviderName(providerName)),
		Default:          compozyconfig.CanonicalProviderName(cfg.Defaults.Provider) == providerName,
		Enabled:          !cfg.IsProviderDisabled(providerName),
		AuthStatus:       authStatus,
	}, nil
}

func (h *BaseHandlers) providerProbeEnv(
	providerName string,
	provider compozyconfig.ProviderConfig,
) (authproviders.ProbeEnv, error) {
	return authproviders.DaemonProbeEnv(authproviders.DaemonProbeEnvOptions{
		HomePaths:      h.HomePaths,
		Vault:          h.Vault,
		ResolveCommand: h.ProviderAuthCommandResolver,
		RunCommand:     h.ProviderAuthRunner,
	}, providerName, provider)
}

func resolveProvider(
	cfg *compozyconfig.Config,
	providerRef string,
) (string, compozyconfig.ProviderConfig, error) {
	providerName := compozyconfig.CanonicalProviderName(providerRef)
	if providerName == "" {
		return "", compozyconfig.ProviderConfig{}, errProviderNotFound
	}
	if !providerExists(cfg, providerName) {
		return "", compozyconfig.ProviderConfig{}, fmt.Errorf("%w: %q", errProviderNotFound, providerName)
	}
	provider, err := cfg.ResolveProvider(providerName)
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
