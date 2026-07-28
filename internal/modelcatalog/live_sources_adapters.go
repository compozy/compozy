package modelcatalog

import (
	"fmt"

	"net/url"

	"strings"
	"time"
)

type liveDiscoveryKind string

const (
	liveDiscoveryNone    liveDiscoveryKind = ""
	liveDiscoveryHTTP    liveDiscoveryKind = "http"
	liveDiscoveryCommand liveDiscoveryKind = "command"
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
	authScheme        liveAuthScheme
	authRequired      bool
	credentialEnvKeys []string
	headers           map[string]string
}

type liveDiscoveryTarget struct {
	kind     liveDiscoveryKind
	endpoint string
	command  string
	timeout  time.Duration
}

var liveProviderAdapters = map[string]liveProviderAdapter{
	liveSourcesCodexKey: {
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://api.openai.com/v1/models",
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
		defaultKind:       liveDiscoveryHTTP,
		defaultEndpoint:   "https://api.anthropic.com/v1/models",
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
	"openclaw": {
		defaultKind: liveDiscoveryNone,
	},
	liveSourcesHermesKey: {
		defaultKind: liveDiscoveryNone,
	},
	"pi": {
		defaultKind: liveDiscoveryNone,
	},
}

func (s *LiveProviderSource) discoveryTarget() (liveDiscoveryTarget, error) {
	discovery := s.provider.Models.Discovery
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
			"model catalog: provider %q has no configured side-effect-free model discovery command or endpoint",
			s.providerID,
		)
	}
	timeout, err := s.discoveryTimeout(discovery.Timeout)
	if err != nil {
		return liveDiscoveryTarget{}, err
	}
	if configuredEndpoint != "" {
		return liveDiscoveryTarget{kind: liveDiscoveryHTTP, endpoint: configuredEndpoint, timeout: timeout}, nil
	}
	if configuredCommand != "" {
		return liveDiscoveryTarget{kind: liveDiscoveryCommand, command: configuredCommand, timeout: timeout}, nil
	}
	switch s.adapter.defaultKind {
	case liveDiscoveryHTTP:
		return liveDiscoveryTarget{
			kind:     liveDiscoveryHTTP,
			endpoint: s.defaultEndpoint(),
			timeout:  timeout,
		}, nil
	case liveDiscoveryCommand:
		return liveDiscoveryTarget{
			kind:    liveDiscoveryCommand,
			command: s.adapter.defaultCommand,
			timeout: timeout,
		}, nil
	default:
		return liveDiscoveryTarget{}, fmt.Errorf(
			"model catalog: provider %q has no configured side-effect-free model discovery command or endpoint",
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

func (s *LiveProviderSource) defaultEndpoint() string {
	baseURL := strings.TrimSpace(s.provider.BaseURL)
	if baseURL == "" {
		return s.adapter.defaultEndpoint
	}
	return joinEndpoint(baseURL, defaultEndpointPath(s.adapter.defaultEndpoint))
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
