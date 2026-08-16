package mcppolicy

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
)

// ValidateRemoteURL requires an absolute HTTP(S) endpoint and restricts plain
// HTTP to the exact localhost name or an IP loopback literal.
func ValidateRemoteURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("parse MCP endpoint URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("MCP endpoint URL must use http or https")
	}
	if parsed.Host == "" || parsed.Hostname() == "" {
		return errors.New("MCP endpoint URL must be absolute")
	}
	if parsed.User != nil {
		return errors.New("MCP endpoint URL must not contain user information")
	}
	if parsed.Fragment != "" || strings.Contains(raw, "#") {
		return errors.New("MCP endpoint URL must not contain a fragment")
	}
	if parsed.Scheme == "http" && !isLoopbackHost(parsed.Hostname()) {
		return errors.New("non-loopback MCP endpoints must use https")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	address := net.ParseIP(host)
	return address != nil && address.IsLoopback()
}
