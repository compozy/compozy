package marketplace

import (
	"fmt"
	"net/url"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
)

type mcpEntry struct {
	entryCommon
	Transport    string        `json:"transport"`
	Command      string        `json:"command,omitempty"`
	Args         []string      `json:"args,omitempty"`
	URL          string        `json:"url,omitempty"`
	OAuth        *mcpOAuth     `json:"oauth,omitempty"`
	Env          []mcpEnvField `json:"env,omitempty"`
	DefaultScope string        `json:"default_scope,omitempty"`
}

type mcpOAuth struct {
	IssuerURL        string   `json:"issuer_url,omitempty"`
	AuthorizationURL string   `json:"authorization_url,omitempty"`
	TokenURL         string   `json:"token_url,omitempty"`
	ClientID         string   `json:"client_id"`
	Scopes           []string `json:"scopes,omitempty"`
}

type mcpEnvField struct {
	Name     string `json:"name"`
	Prompt   string `json:"prompt,omitempty"`
	Required bool   `json:"required"`
	Secret   bool   `json:"secret"`
	Default  string `json:"default,omitempty"`
}

func decodeMCPEntry(raw []byte) (Entry, error) {
	var value mcpEntry
	if err := decodeStrict(raw, &value); err != nil {
		return Entry{}, err
	}
	if err := value.validate(); err != nil {
		return Entry{}, err
	}
	return commonEntry(KindMCP, value.entryCommon, value)
}

func (e mcpEntry) validate() error {
	if err := e.entryCommon.validate(KindMCP); err != nil {
		return err
	}
	if err := e.validateTransport(); err != nil {
		return err
	}
	if err := e.validateEnvironment(); err != nil {
		return err
	}
	return e.validateDefaultScope()
}

func (e mcpEntry) validateTransport() error {
	transport := strings.TrimSpace(e.Transport)
	switch transport {
	case "stdio":
		if strings.TrimSpace(e.Command) == "" {
			return fmt.Errorf("marketplace catalog MCP entry %q command is required for stdio", e.EntryID)
		}
		if strings.TrimSpace(e.URL) != "" {
			return fmt.Errorf("marketplace catalog MCP entry %q must not set url for stdio", e.EntryID)
		}
		if e.OAuth != nil {
			return fmt.Errorf("marketplace catalog MCP entry %q must not set oauth for stdio", e.EntryID)
		}
	case protocolHTTP, "sse":
		if strings.TrimSpace(e.URL) == "" {
			return fmt.Errorf("marketplace catalog MCP entry %q url is required for %s", e.EntryID, transport)
		}
		if strings.TrimSpace(e.Command) != "" || len(e.Args) > 0 {
			return fmt.Errorf(
				"marketplace catalog MCP entry %q must not set command or args for %s",
				e.EntryID,
				transport,
			)
		}
		if len(e.Env) > 0 {
			return fmt.Errorf("marketplace catalog MCP entry %q must not set env for %s", e.EntryID, transport)
		}
		if err := validateAbsoluteHTTPURL(e.URL, "url"); err != nil {
			return fmt.Errorf("marketplace catalog MCP entry %q: %w", e.EntryID, err)
		}
		if err := e.validateOAuth(); err != nil {
			return err
		}
	default:
		return fmt.Errorf("marketplace catalog MCP entry %q transport must be stdio, http, or sse", e.EntryID)
	}
	return nil
}

func (e mcpEntry) validateOAuth() error {
	if e.OAuth == nil {
		return nil
	}
	oauth := e.OAuth
	if strings.TrimSpace(oauth.ClientID) == "" {
		return fmt.Errorf("marketplace catalog MCP entry %q oauth.client_id is required", e.EntryID)
	}
	issuerURL := strings.TrimSpace(oauth.IssuerURL)
	authorizationURL := strings.TrimSpace(oauth.AuthorizationURL)
	tokenURL := strings.TrimSpace(oauth.TokenURL)
	if issuerURL == "" && (authorizationURL == "" || tokenURL == "") {
		return fmt.Errorf(
			"marketplace catalog MCP entry %q oauth requires issuer_url or both authorization_url and token_url",
			e.EntryID,
		)
	}
	for _, endpoint := range []struct {
		name  string
		value string
	}{
		{name: "issuer_url", value: issuerURL},
		{name: "authorization_url", value: authorizationURL},
		{name: "token_url", value: tokenURL},
	} {
		if endpoint.value == "" {
			continue
		}
		if err := aghconfig.ValidateMCPOAuthURL("oauth."+endpoint.name, endpoint.value); err != nil {
			return fmt.Errorf("marketplace catalog MCP entry %q: %w", e.EntryID, err)
		}
	}
	return nil
}

func (e mcpEntry) validateEnvironment() error {
	seenEnv := make(map[string]struct{}, len(e.Env))
	for index, field := range e.Env {
		name := strings.TrimSpace(field.Name)
		if err := aghconfig.ValidateMCPStdioEnvName("env", name, field.Secret); err != nil {
			return fmt.Errorf("marketplace catalog MCP entry %q env[%d]: %w", e.EntryID, index, err)
		}
		if _, exists := seenEnv[name]; exists {
			return fmt.Errorf("marketplace catalog MCP entry %q env name %q is duplicated", e.EntryID, name)
		}
		seenEnv[name] = struct{}{}
		if field.Secret && strings.TrimSpace(field.Default) != "" {
			return fmt.Errorf(
				"marketplace catalog MCP entry %q env[%d] must not set default for secret fields",
				e.EntryID,
				index,
			)
		}
	}
	return nil
}

func (e mcpEntry) validateDefaultScope() error {
	if scope := strings.TrimSpace(e.DefaultScope); scope != "" && scope != "workspace" && scope != "global" {
		return fmt.Errorf("marketplace catalog MCP entry %q default_scope must be workspace or global", e.EntryID)
	}
	return nil
}

func validateAbsoluteHTTPURL(raw string, field string) error {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || parsed.Host == "" || (parsed.Scheme != protocolHTTP && parsed.Scheme != protocolHTTPS) {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL", field)
	}
	if parsed.User != nil {
		return fmt.Errorf("%s must be an absolute HTTP(S) URL without credentials", field)
	}
	return nil
}
