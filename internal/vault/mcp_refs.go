package vault

import (
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var mcpSafeSegmentPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)

const (
	// MCPGlobalScope identifies daemon-global MCP credentials.
	MCPGlobalScope = "global"
	// MCPWorkspaceScope identifies workspace-owned MCP credentials.
	MCPWorkspaceScope = "workspace"

	mcpEncodedSegmentPrefix = "encoded-"
)

// MCPSecretOwnerPrefix returns the canonical scope-qualified Vault prefix for
// one MCP server. The returned value always ends with a slash.
func MCPSecretOwnerPrefix(scope string, workspaceID string, serverName string) (string, error) {
	serverSegment, err := MCPServerSegment(serverName)
	if err != nil {
		return "", err
	}

	var owner string
	switch strings.TrimSpace(scope) {
	case MCPGlobalScope:
		if strings.TrimSpace(workspaceID) != "" {
			return "", errors.New("vault: global MCP secret owner cannot include workspace_id")
		}
		owner = "global/" + serverSegment
	case MCPWorkspaceScope:
		segment, err := MCPWorkspaceSegment(workspaceID)
		if err != nil {
			return "", err
		}
		owner = "ws/" + segment + "/" + serverSegment
	default:
		return "", fmt.Errorf("vault: unsupported MCP secret scope %q", strings.TrimSpace(scope))
	}

	ref := "vault:mcp/" + owner + "/"
	if err := ValidateSecretRef(ref + "value"); err != nil {
		return "", fmt.Errorf("vault: build MCP secret owner prefix: %w", err)
	}
	return ref, nil
}

// MCPServerSegment returns a collision-safe Vault path segment for an MCP
// server name while preserving the exact server name as the runtime identity.
func MCPServerSegment(serverName string) (string, error) {
	normalized := strings.TrimSpace(serverName)
	if normalized == "" || strings.Contains(normalized, "/") || strings.ContainsRune(normalized, '\x00') {
		return "", fmt.Errorf("vault: invalid MCP secret owner %q", normalized)
	}
	return collisionSafeMCPSegment(normalized), nil
}

// MCPWorkspaceSegment returns a collision-safe Vault path segment for a
// workspace identifier. Grammar-safe identifiers remain readable; reserved or
// unsafe values use a deterministic hex encoding.
func MCPWorkspaceSegment(workspaceID string) (string, error) {
	normalized := strings.TrimSpace(workspaceID)
	if normalized == "" {
		return "", errors.New("vault: workspace_id is required for workspace MCP secrets")
	}
	return collisionSafeMCPSegment(normalized), nil
}

func collisionSafeMCPSegment(normalized string) string {
	if !strings.HasPrefix(normalized, mcpEncodedSegmentPrefix) && mcpSafeSegmentPattern.MatchString(normalized) {
		return normalized
	}
	return mcpEncodedSegmentPrefix + hex.EncodeToString([]byte(normalized))
}
