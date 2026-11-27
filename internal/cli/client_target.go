package cli

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

const (
	clientTargetLocal = "local"
	clientUnixNetwork = "unix"
)

// ClientTarget describes one resolved daemon transport without exposing credentials in config.
type ClientTarget struct {
	kind       string
	socketPath string
	computerID string
}

// LocalClientTarget resolves the unchanged HTTP-over-UDS transport.
func LocalClientTarget(socketPath string) ClientTarget {
	return ClientTarget{kind: clientTargetLocal, socketPath: strings.TrimSpace(socketPath)}
}

func (t ClientTarget) validate() error {
	if t.kind != clientTargetLocal || strings.TrimSpace(t.socketPath) == "" {
		return errors.New("cli: daemon socket path is required")
	}
	return nil
}

func (t ClientTarget) requestBaseURL() string {
	if t.computerID != "" {
		return baseURL + "/c/" + url.PathEscape(t.computerID)
	}
	return baseURL
}

func (t ClientTarget) displayName() string {
	return t.socketPath
}

func (t ClientTarget) websocketBaseURL() (string, error) {
	parsed, err := url.Parse(t.requestBaseURL())
	if err != nil {
		return "", fmt.Errorf("cli: parse connection target: %w", err)
	}
	switch parsed.Scheme {
	case mcpServeTransportHTTP:
		parsed.Scheme = "ws"
	default:
		return "", fmt.Errorf("cli: unsupported connection target scheme %q", parsed.Scheme)
	}
	return strings.TrimSuffix(parsed.String(), "/"), nil
}
