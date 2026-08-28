package modelcatalog

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
)

type liveDiscoveryKind string

const (
	liveDiscoveryNone    liveDiscoveryKind = ""
	liveDiscoveryHTTP    liveDiscoveryKind = "http"
	liveDiscoveryCommand liveDiscoveryKind = "command"
	liveDiscoveryACP     liveDiscoveryKind = "acp"
	cursorModelsCommand                    = "cursor-agent models"
)

type liveAuthScheme string

const (
	liveAuthNone      liveAuthScheme = ""
	liveAuthBearer    liveAuthScheme = "bearer"
	liveAuthAnthropic liveAuthScheme = "anthropic"
	liveAuthGemini    liveAuthScheme = "gemini"
)

type liveProviderAdapter struct {
	defaultKind       liveDiscoveryKind
	defaultEndpoint   string
	defaultCommand    string
	bootstrapOnList   bool
	commandOnly       bool
	parseCommandRows  liveCommandRowsParser
	parseACPModelRows liveACPModelRowsParser
	authScheme        liveAuthScheme
	authRequired      bool
	credentialEnvKeys []string
	headers           map[string]string
}

type liveCommandRowsParser func(string, string, time.Time) ([]ModelRow, error)

type liveACPModelRowsParser func(
	string,
	compozyconfig.ProviderModelsConfig,
	acp.SessionConfigOption,
	time.Time,
) []ModelRow

type liveDiscoveryTarget struct {
	kind     liveDiscoveryKind
	endpoint string
	command  string
	timeout  time.Duration
}

var liveProviderAdapters = map[string]liveProviderAdapter{
	liveSourcesCodexKey: {
		defaultKind:       liveDiscoveryACP,
		bootstrapOnList:   true,
		authScheme:        liveAuthBearer,
		authRequired:      true,
		credentialEnvKeys: []string{liveSourcesOpenAIEnvName},
	},
	liveSourcesOpenaiKey: {
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://api.openai.com/v1/models",
		authScheme:        liveAuthBearer,
		authRequired:      true,
		credentialEnvKeys: []string{liveSourcesOpenAIEnvName},
	},
	liveSourcesClaudeKey: {
		defaultKind:       liveDiscoveryACP,
		bootstrapOnList:   true,
		parseACPModelRows: parseClaudeModelRows,
		authScheme:        liveAuthAnthropic,
		authRequired:      true,
		credentialEnvKeys: []string{"ANTHROPIC_API_KEY"},
		headers:           map[string]string{"anthropic-version": "2023-06-01"},
	},
	"anthropic": {
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://api.anthropic.com/v1/models",
		authScheme:        liveAuthAnthropic,
		authRequired:      true,
		credentialEnvKeys: []string{"ANTHROPIC_API_KEY"},
		headers:           map[string]string{"anthropic-version": "2023-06-01"},
	},
	string(liveAuthGemini): {
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://generativelanguage.googleapis.com/v1beta/models",
		authScheme:        liveAuthGemini,
		authRequired:      true,
		credentialEnvKeys: []string{"GEMINI_API_KEY", "GOOGLE_API_KEY"},
	},
	liveSourcesOpenrouterKey: {
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://openrouter.ai/api/v1/models",
		authScheme:        liveAuthBearer,
		authRequired:      true,
		credentialEnvKeys: []string{"OPENROUTER_API_KEY"},
	},
	liveSourcesVercelAIGatewayValue: {
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://ai-gateway.vercel.sh/v1/models",
		authScheme:        liveAuthBearer,
		authRequired:      false,
		credentialEnvKeys: []string{"AI_GATEWAY_API_KEY", "VERCEL_AI_GATEWAY_API_KEY"},
	},
	liveSourcesOllamaKey: {
		defaultKind:     liveDiscoveryHTTP,
		defaultEndpoint: "http://localhost:11434/api/tags",
	},
	liveSourcesOpencodeKey: {
		defaultKind:    liveDiscoveryCommand,
		defaultCommand: "opencode models",
	},
	liveSourcesCursorKey: {
		defaultKind:      liveDiscoveryCommand,
		defaultCommand:   cursorModelsCommand,
		bootstrapOnList:  true,
		parseCommandRows: parseCursorModelRows,
		commandOnly:      true,
	},
	"openclaw": {
		defaultKind: liveDiscoveryNone,
	},
	liveSourcesHermesKey: {
		defaultKind:     liveDiscoveryACP,
		bootstrapOnList: true,
	},
	"pi": {
		defaultKind: liveDiscoveryNone,
	},
}

func (s *LiveProviderSource) discoveryTarget(
	provider compozyconfig.ProviderConfig,
) (liveDiscoveryTarget, error) {
	discovery := provider.Models.Discovery
	configuredCommand := strings.TrimSpace(discovery.Command)
	configuredEndpoint := strings.TrimSpace(discovery.Endpoint)
	hasConfiguredPath := configuredCommand != "" || configuredEndpoint != ""
	if discovery.Enabled != nil && !*discovery.Enabled {
		return liveDiscoveryTarget{}, ErrSourceDisabled
	}
	if s.adapter.defaultKind == liveDiscoveryNone && discovery.Enabled == nil {
		if hasConfiguredPath {
			return liveDiscoveryTarget{}, ErrSourceDisabled
		}
		return liveDiscoveryTarget{}, fmt.Errorf(
			"model catalog: provider %q has no configured model discovery path",
			s.providerID,
		)
	}
	timeout, err := s.discoveryTimeout(discovery.Timeout)
	if err != nil {
		return liveDiscoveryTarget{}, err
	}
	if configuredEndpoint != "" {
		if s.adapter.commandOnly {
			return liveDiscoveryTarget{}, fmt.Errorf(
				"model catalog: provider %q requires a model discovery command, not an endpoint",
				s.providerID,
			)
		}
		return liveDiscoveryTarget{kind: liveDiscoveryHTTP, endpoint: configuredEndpoint, timeout: timeout}, nil
	}
	if configuredCommand != "" {
		kind := liveDiscoveryCommand
		if s.adapter.defaultKind == liveDiscoveryACP {
			kind = liveDiscoveryACP
		}
		return liveDiscoveryTarget{kind: kind, command: configuredCommand, timeout: timeout}, nil
	}
	switch s.adapter.defaultKind {
	case liveDiscoveryHTTP:
		return liveDiscoveryTarget{
			kind:     liveDiscoveryHTTP,
			endpoint: s.defaultEndpoint(provider),
			timeout:  timeout,
		}, nil
	case liveDiscoveryCommand:
		return liveDiscoveryTarget{
			kind:    liveDiscoveryCommand,
			command: s.adapter.defaultCommand,
			timeout: timeout,
		}, nil
	case liveDiscoveryACP:
		command := strings.TrimSpace(provider.Command)
		if command == "" {
			command = s.adapter.defaultCommand
		}
		return liveDiscoveryTarget{
			kind:    liveDiscoveryACP,
			command: command,
			timeout: timeout,
		}, nil
	default:
		return liveDiscoveryTarget{}, fmt.Errorf(
			"model catalog: provider %q has no configured model discovery path",
			s.providerID,
		)
	}
}

func (s *LiveProviderSource) discoveryTimeout(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return s.defaultTimeout, nil
	}
	timeout, err := time.ParseDuration(trimmed)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("model catalog: provider %q discovery timeout must be a positive duration", s.providerID)
	}
	return timeout, nil
}

func (s *LiveProviderSource) defaultEndpoint(provider compozyconfig.ProviderConfig) string {
	baseURL := strings.TrimSpace(provider.BaseURL)
	if baseURL == "" {
		return s.adapter.defaultEndpoint
	}
	return joinEndpoint(baseURL, defaultEndpointPath(s.adapter.defaultEndpoint))
}

// CatalogExecutionFingerprint identifies the effective live discovery invocation without storing secrets.
func (s *LiveProviderSource) CatalogExecutionFingerprint() (string, error) {
	if s == nil {
		return "", fmt.Errorf("model catalog: live provider source is required")
	}
	provider := s.providerSnapshot()
	target, err := s.discoveryTarget(provider)
	if err != nil {
		target = s.discoveryFingerprintTarget(provider, err)
	}
	command := strings.TrimSpace(target.command)
	if command != "" {
		bin, args, parseErr := parseDiscoveryCommand(command)
		if parseErr == nil {
			command = strings.Join(append([]string{bin}, args...), "\x1f")
		}
	}
	credentialShape := make([]string, 0, len(provider.EffectiveCredentialSlots()))
	for _, slot := range provider.EffectiveCredentialSlots() {
		credentialShape = append(credentialShape, strings.Join([]string{
			strings.TrimSpace(slot.Name),
			strings.TrimSpace(slot.TargetEnv),
			fmt.Sprintf("%t", slot.Required),
		}, "\x1f"))
	}
	modelMappings := ""
	if s.adapter.parseACPModelRows != nil {
		modelMappings = providerModelMappingFingerprint(provider.Models)
	}
	return CatalogExecutionFingerprint(
		s.providerID,
		string(target.kind),
		strings.TrimSpace(target.endpoint),
		command,
		strings.TrimSpace(s.workingDir),
		string(provider.EffectiveHarness()),
		string(provider.EffectiveAuthMode()),
		string(provider.EffectiveEnvPolicy()),
		string(provider.EffectiveHomePolicy()),
		strings.Join(credentialShape, "\x1e"),
		modelMappings,
	), nil
}

func providerModelMappingFingerprint(models compozyconfig.ProviderModelsConfig) string {
	modelIDs := make([]string, 0, len(models.Curated)+1)
	modelIDs = append(modelIDs, strings.TrimSpace(models.Default))
	for _, model := range models.Curated {
		modelIDs = append(modelIDs, strings.TrimSpace(model.ID))
	}
	slices.Sort(modelIDs)
	return strings.Join(modelIDs, "\x1e")
}

func (s *LiveProviderSource) discoveryFingerprintTarget(
	provider compozyconfig.ProviderConfig,
	discoveryErr error,
) liveDiscoveryTarget {
	discovery := provider.Models.Discovery
	target := liveDiscoveryTarget{
		kind:     s.adapter.defaultKind,
		endpoint: strings.TrimSpace(discovery.Endpoint),
		command:  strings.TrimSpace(discovery.Command),
	}
	if target.endpoint != "" {
		target.kind = liveDiscoveryHTTP
	}
	if target.command != "" {
		target.kind = liveDiscoveryCommand
		if s.adapter.defaultKind == liveDiscoveryACP {
			target.kind = liveDiscoveryACP
		}
	}
	if target.endpoint == "" && target.command == "" {
		switch target.kind {
		case liveDiscoveryHTTP:
			target.endpoint = s.defaultEndpoint(provider)
		case liveDiscoveryCommand:
			target.command = s.adapter.defaultCommand
		case liveDiscoveryACP:
			target.command = strings.TrimSpace(provider.Command)
			if target.command == "" {
				target.command = s.adapter.defaultCommand
			}
		}
	}
	if errors.Is(discoveryErr, ErrSourceDisabled) {
		target.kind = liveDiscoveryKind("disabled:" + string(target.kind))
	} else {
		target.kind = liveDiscoveryKind("invalid:" + string(target.kind))
	}
	return target
}

func joinEndpoint(baseURL string, path string) string {
	trimmedBase := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmedBase == "" {
		return path
	}
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return trimmedBase
	}
	if parsed, err := url.Parse(trimmedBase); err == nil {
		basePath := strings.TrimRight(parsed.Path, "/")
		switch {
		case strings.HasSuffix(basePath, "/v1") && strings.HasPrefix(trimmedPath, "/v1/"):
			trimmedPath = strings.TrimPrefix(trimmedPath, "/v1")
		case strings.HasSuffix(basePath, "/v1beta") && strings.HasPrefix(trimmedPath, "/v1beta/"):
			trimmedPath = strings.TrimPrefix(trimmedPath, "/v1beta")
		case strings.HasSuffix(basePath, "/api/v1") && strings.HasPrefix(trimmedPath, "/api/v1/"):
			trimmedPath = strings.TrimPrefix(trimmedPath, "/api/v1")
		case strings.HasSuffix(basePath, "/api") && strings.HasPrefix(trimmedPath, "/api/"):
			trimmedPath = strings.TrimPrefix(trimmedPath, "/api")
		}
	}
	if strings.HasPrefix(trimmedPath, "/") {
		return trimmedBase + trimmedPath
	}
	return trimmedBase + "/" + trimmedPath
}
