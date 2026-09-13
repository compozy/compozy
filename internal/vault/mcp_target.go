package vault

import (
	"errors"
	"strings"
)

const MCPManualOwner = "manual"

// MCPSecretTarget carries only the identity needed to address MCP credentials.
// It keeps the Vault boundary independent of the OAuth/config packages.
type MCPSecretTarget struct {
	Scope       string
	WorkspaceID string
	ServerName  string
	Owner       string
}

// NormalizeMCPOwner maps omitted owner fields from released callers to manual.
func NormalizeMCPOwner(owner string) string {
	owner = strings.TrimSpace(owner)
	if owner == "" {
		return MCPManualOwner
	}
	return owner
}

func ValidateMCPOwner(owner string) error {
	owner = NormalizeMCPOwner(owner)
	if owner == MCPManualOwner {
		return nil
	}
	name, found := strings.CutPrefix(owner, "extension:")
	if !found || strings.TrimSpace(name) == "" || strings.TrimSpace(name) != name ||
		strings.ContainsAny(name, "/\x00") {
		return errors.New("vault: MCP owner must be manual or extension:<name> without slash or NUL")
	}
	return nil
}
