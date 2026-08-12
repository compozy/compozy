package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type ForgeRuntime interface {
	ForgeCapabilities(context.Context, []string) (extensioncontract.ForgeCapabilitiesResponse, error)
	ForgeStatus(context.Context, extensioncontract.ForgeStatusRequest) (extensioncontract.ForgeStatusResponse, error)
	ForgeCreatePR(context.Context, extensioncontract.ForgePRCreateRequest) (extensioncontract.ForgePRCreateResponse, error)
}

type ForgeProviderError struct {
	cause       string
	unavailable bool
}

func (e *ForgeProviderError) Error() string {
	if e.unavailable {
		return fmt.Sprintf("extension: forge provider unavailable (%s)", e.cause)
	}
	return fmt.Sprintf("extension: forge provider failed (%s)", e.cause)
}

func (e *ForgeProviderError) Unwrap() error {
	if e.unavailable {
		return toolspkg.ErrToolUnavailable
	}
	return nil
}

func ForgeProviderErrorCause(err error) string {
	var failure *ForgeProviderError
	if errors.As(err, &failure) {
		return failure.cause
	}
	return ""
}

var _ ForgeRuntime = (*Manager)(nil)

func (m *Manager) ForgeCapabilities(
	ctx context.Context,
	remoteURLs []string,
) (extensioncontract.ForgeCapabilitiesResponse, error) {
	remotes, err := normalizeForgeRemoteURLs(remoteURLs)
	if err != nil {
		return extensioncontract.ForgeCapabilitiesResponse{}, err
	}
	names, err := m.forgeProviderNames(ctx)
	if err != nil {
		return extensioncontract.ForgeCapabilitiesResponse{}, err
	}
	var providerErrors error
	for _, name := range names {
		process, resolvedName, processErr := m.extensionServiceProcess(
			ctx, name, extensionprotocol.ExtensionServiceMethodForgeCapabilities,
		)
		if processErr != nil {
			providerErrors = errors.Join(providerErrors, processErr)
			continue
		}
		var response extensioncontract.ForgeCapabilitiesResponse
		if callErr := process.Call(
			ctx,
			string(extensionprotocol.ExtensionServiceMethodForgeCapabilities),
			extensioncontract.ForgeCapabilitiesRequest{RemoteURLs: remotes},
			&response,
		); callErr != nil {
			providerErrors = errors.Join(
				providerErrors,
				fmt.Errorf("extension: forge capabilities via %q: %w", resolvedName, callErr),
			)
			continue
		}
		if !response.Served {
			continue
		}
		response.Winner = resolvedName
		response.TemplatePaths = slices.Clone(response.TemplatePaths)
		return response, validateForgeCapabilitiesResponse(response)
	}
	if providerErrors != nil {
		return extensioncontract.ForgeCapabilitiesResponse{}, providerErrors
	}
	return extensioncontract.ForgeCapabilitiesResponse{
		Cause: extensioncontract.ForgeCauseUnsupportedRemote,
	}, nil
}

func (m *Manager) ForgeStatus(
	ctx context.Context,
	request extensioncontract.ForgeStatusRequest,
) (extensioncontract.ForgeStatusResponse, error) {
	remotes, err := normalizeForgeRemoteURLs(request.RemoteURLs)
	if err != nil {
		return extensioncontract.ForgeStatusResponse{}, err
	}
	request.RemoteURLs = remotes
	if strings.TrimSpace(request.Branch) == "" {
		return extensioncontract.ForgeStatusResponse{}, errors.New("extension: forge branch is required")
	}
	capabilities, err := m.ForgeCapabilities(ctx, request.RemoteURLs)
	if err != nil {
		return extensioncontract.ForgeStatusResponse{}, err
	}
	if err := requireAvailableForgeProvider(capabilities); err != nil {
		return extensioncontract.ForgeStatusResponse{}, err
	}
	process, name, err := m.extensionServiceProcess(
		ctx, capabilities.Winner, extensionprotocol.ExtensionServiceMethodForgeStatus,
	)
	if err != nil {
		return extensioncontract.ForgeStatusResponse{}, err
	}
	var response extensioncontract.ForgeStatusResponse
	if err := process.Call(ctx, string(extensionprotocol.ExtensionServiceMethodForgeStatus), request, &response); err != nil {
		return extensioncontract.ForgeStatusResponse{}, fmt.Errorf("extension: forge status via %q: %w", name, err)
	}
	if response.Provider == "" {
		response.Provider = name
	}
	if response.Cause != "" {
		return extensioncontract.ForgeStatusResponse{}, forgeProviderFailure(response.Cause)
	}
	if err := validateForgeStatusResponse(response); err != nil {
		return extensioncontract.ForgeStatusResponse{}, fmt.Errorf("extension: forge status via %q: %w", name, err)
	}
	return response, nil
}

func (m *Manager) ForgeCreatePR(
	ctx context.Context,
	request extensioncontract.ForgePRCreateRequest,
) (extensioncontract.ForgePRCreateResponse, error) {
	remotes, err := normalizeForgeRemoteURLs(request.RemoteURLs)
	if err != nil {
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	request.RemoteURLs = remotes
	if strings.TrimSpace(request.Head) == "" || strings.TrimSpace(request.Base) == "" ||
		strings.TrimSpace(request.Title) == "" {
		return extensioncontract.ForgePRCreateResponse{}, errors.New("extension: forge head, base, and title are required")
	}
	capabilities, err := m.ForgeCapabilities(ctx, request.RemoteURLs)
	if err != nil {
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	if err := requireAvailableForgeProvider(capabilities); err != nil {
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	process, name, err := m.extensionServiceProcess(
		ctx, capabilities.Winner, extensionprotocol.ExtensionServiceMethodForgePRCreate,
	)
	if err != nil {
		return extensioncontract.ForgePRCreateResponse{}, err
	}
	var response extensioncontract.ForgePRCreateResponse
	if err := process.Call(ctx, string(extensionprotocol.ExtensionServiceMethodForgePRCreate), request, &response); err != nil {
		return extensioncontract.ForgePRCreateResponse{}, fmt.Errorf("extension: forge PR create via %q: %w", name, err)
	}
	if response.Cause != "" {
		return extensioncontract.ForgePRCreateResponse{}, forgeProviderFailure(response.Cause)
	}
	if response.Status != "created" && response.Status != "opened_existing" {
		return extensioncontract.ForgePRCreateResponse{}, fmt.Errorf(
			"extension: forge PR create via %q returned invalid status %q", name, response.Status,
		)
	}
	parsedURL, parseErr := url.ParseRequestURI(strings.TrimSpace(response.URL))
	if response.Number <= 0 || parseErr != nil || parsedURL.Host == "" || parsedURL.Scheme == "" {
		return extensioncontract.ForgePRCreateResponse{}, fmt.Errorf(
			"extension: forge PR create via %q returned an invalid pull request", name,
		)
	}
	return response, nil
}

func (m *Manager) forgeProviderNames(ctx context.Context) ([]string, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrManagerRequired
	}
	m.mu.RLock()
	type candidate struct {
		name      string
		startedAt time.Time
	}
	candidates := make([]candidate, 0)
	for name, ext := range m.extensions {
		if ext == nil || !ext.active || ext.process == nil || ext.initialize == nil {
			continue
		}
		provides := ext.info.Capabilities.Provides
		if ext.manifest != nil && len(ext.manifest.Capabilities.Provides) > 0 {
			provides = ext.manifest.Capabilities.Provides
		}
		if slices.Contains(provides, extensionprotocol.CapabilityProvideForgeProvider) {
			candidates = append(candidates, candidate{name: name, startedAt: ext.lastStartedAt})
		}
	}
	m.mu.RUnlock()
	slices.SortFunc(candidates, func(left, right candidate) int {
		if order := left.startedAt.Compare(right.startedAt); order != 0 {
			return order
		}
		return strings.Compare(left.name, right.name)
	})
	names := make([]string, len(candidates))
	for index, candidate := range candidates {
		names[index] = candidate.name
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("extension: no active forge provider: %w", toolspkg.ErrToolUnavailable)
	}
	return names, nil
}

func normalizeForgeRemoteURLs(values []string) ([]string, error) {
	normalized := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil, errors.New("extension: at least one forge remote URL is required")
	}
	return normalized, nil
}

func validateForgeCapabilitiesResponse(value extensioncontract.ForgeCapabilitiesResponse) error {
	if !value.Served {
		return nil
	}
	if strings.TrimSpace(value.Winner) == "" || strings.TrimSpace(value.Provider) == "" ||
		strings.TrimSpace(value.ServedRemote) == "" || strings.TrimSpace(value.RequestNoun) == "" ||
		strings.TrimSpace(value.OpenActionLabel) == "" || strings.TrimSpace(value.ViewActionLabel) == "" {
		return errors.New("extension: served forge capabilities are incomplete")
	}
	if value.Cause != "" && !validForgeProviderCause(value.Cause) {
		return fmt.Errorf("extension: forge capabilities returned invalid cause %q", value.Cause)
	}
	if value.Available && value.CredentialSource != "binding" && value.CredentialSource != "gh" {
		return errors.New("extension: available forge provider has invalid credential source")
	}
	return nil
}

func requireAvailableForgeProvider(value extensioncontract.ForgeCapabilitiesResponse) error {
	if value.Served && value.Available && value.Winner != "" {
		return nil
	}
	cause := strings.TrimSpace(value.Cause)
	if cause == "" {
		cause = extensioncontract.ForgeCauseUnsupportedRemote
	}
	return forgeProviderFailure(cause)
}

func forgeProviderFailure(cause string) error {
	value := strings.TrimSpace(cause)
	if value == "" {
		value = extensioncontract.ForgeCauseUnsupportedRemote
	}
	if !validForgeProviderCause(value) {
		return fmt.Errorf("extension: forge provider returned invalid cause %q", value)
	}
	unavailable := value == extensioncontract.ForgeCauseUnsupportedRemote ||
		value == extensioncontract.ForgeCauseCredentialAbsent
	return &ForgeProviderError{cause: value, unavailable: unavailable}
}

func validForgeProviderCause(value string) bool {
	switch strings.TrimSpace(value) {
	case extensioncontract.ForgeCauseRateLimited,
		extensioncontract.ForgeCauseCredentialExpired,
		extensioncontract.ForgeCauseUnsupportedRemote,
		extensioncontract.ForgeCauseCredentialAbsent:
		return true
	default:
		return false
	}
}

func validateForgeStatusResponse(value extensioncontract.ForgeStatusResponse) error {
	if value.PRState != nil {
		switch *value.PRState {
		case "open", "closed", "merged":
		default:
			return errors.New("invalid pull request state")
		}
	}
	return nil
}
