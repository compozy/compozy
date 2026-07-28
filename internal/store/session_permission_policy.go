package store

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// SessionPermissionPolicy captures concrete permission atoms available to a session.
type SessionPermissionPolicy struct {
	Tools           []string `json:"tools"`
	Skills          []string `json:"skills"`
	MCPServers      []string `json:"mcp_servers"`
	WorkspacePaths  []string `json:"workspace_paths"`
	NetworkChannels []string `json:"network_channels"`
	SandboxProfiles []string `json:"sandbox_profiles"`
}

// NormalizeSessionPermissionPolicy returns a policy with stable, trimmed atom lists.
func NormalizeSessionPermissionPolicy(policy SessionPermissionPolicy) SessionPermissionPolicy {
	return SessionPermissionPolicy{
		Tools:           normalizePolicyAtoms(policy.Tools),
		Skills:          normalizePolicyAtoms(policy.Skills),
		MCPServers:      normalizePolicyAtoms(policy.MCPServers),
		WorkspacePaths:  normalizePolicyAtoms(policy.WorkspacePaths),
		NetworkChannels: normalizePolicyAtoms(policy.NetworkChannels),
		SandboxProfiles: normalizePolicyAtoms(policy.SandboxProfiles),
	}
}

func validateSessionPermissionPolicy(policy SessionPermissionPolicy) error {
	checks := []struct {
		name   string
		values []string
	}{
		{name: "tools", values: policy.Tools},
		{name: "skills", values: policy.Skills},
		{name: "mcp_servers", values: policy.MCPServers},
		{name: "workspace_paths", values: policy.WorkspacePaths},
		{name: "network_channels", values: policy.NetworkChannels},
		{name: "sandbox_profiles", values: policy.SandboxProfiles},
	}
	for _, check := range checks {
		for idx, value := range check.values {
			if strings.TrimSpace(value) == "" {
				return fmt.Errorf("store: session permission policy %s contains an empty atom", check.name)
			}
			if check.name == "tools" {
				if err := validateCanonicalToolIDAtom(value); err != nil {
					return fmt.Errorf(
						"store: session permission policy tools[%d] must be canonical ToolID: %w",
						idx,
						err,
					)
				}
			}
		}
	}
	return nil
}

// ValidateChildSessionToolSubset ensures child concrete tool atoms cannot exceed the parent.
func ValidateChildSessionToolSubset(parent SessionPermissionPolicy, child SessionPermissionPolicy) error {
	normalizedParent := NormalizeSessionPermissionPolicy(parent)
	normalizedChild := NormalizeSessionPermissionPolicy(child)
	if err := validateSessionPermissionPolicy(normalizedParent); err != nil {
		return fmt.Errorf("store: parent session permission policy invalid: %w", err)
	}
	if err := validateSessionPermissionPolicy(normalizedChild); err != nil {
		return fmt.Errorf("store: child session permission policy invalid: %w", err)
	}

	parentTools := make(map[string]struct{}, len(normalizedParent.Tools))
	for _, tool := range normalizedParent.Tools {
		parentTools[tool] = struct{}{}
	}
	for _, tool := range normalizedChild.Tools {
		if _, ok := parentTools[tool]; !ok {
			return fmt.Errorf("store: child session tool %q exceeds parent permission policy", tool)
		}
	}
	return nil
}

func validateCanonicalToolIDAtom(value string) error {
	switch {
	case value == "":
		return fmt.Errorf("id is required")
	case !strings.Contains(value, "__"):
		return fmt.Errorf("id must include canonical namespace separator")
	case len(value) > 64:
		return fmt.Errorf("id exceeds 64 characters")
	case strings.Contains(value, "___"):
		return fmt.Errorf("reserved separator is ambiguous")
	}
	for segment := range strings.SplitSeq(value, "__") {
		if segment == "" {
			return fmt.Errorf("id contains an empty segment")
		}
		if strings.HasPrefix(segment, "_") || strings.HasSuffix(segment, "_") {
			return fmt.Errorf("segment uses reserved underscore boundary")
		}
		for i, r := range segment {
			if i == 0 {
				if r < 'a' || r > 'z' {
					return fmt.Errorf("segment must start with a lowercase letter")
				}
				continue
			}
			if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
				continue
			}
			return fmt.Errorf("segment contains an unsupported character")
		}
	}
	return nil
}

func normalizePolicyAtoms(values []string) []string {
	if values == nil {
		return []string{}
	}
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		normalized = append(normalized, strings.TrimSpace(value))
	}
	slices.Sort(normalized)
	return slices.Compact(normalized)
}

// EncodeSessionPermissionPolicy marshals normalized permission policy metadata.
func EncodeSessionPermissionPolicy(policy SessionPermissionPolicy) (string, error) {
	normalized := NormalizeSessionPermissionPolicy(policy)
	if err := validateSessionPermissionPolicy(normalized); err != nil {
		return "", err
	}
	data, err := json.Marshal(normalized)
	if err != nil {
		return "", fmt.Errorf("store: marshal session permission policy: %w", err)
	}
	return string(data), nil
}

// DecodeSessionPermissionPolicy unmarshals permission policy metadata from the global session catalog.
func DecodeSessionPermissionPolicy(raw string) (SessionPermissionPolicy, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return NormalizeSessionPermissionPolicy(SessionPermissionPolicy{}), nil
	}
	var policy SessionPermissionPolicy
	if err := json.Unmarshal([]byte(trimmed), &policy); err != nil {
		return SessionPermissionPolicy{}, fmt.Errorf("store: parse session permission policy json: %w", err)
	}
	normalized := NormalizeSessionPermissionPolicy(policy)
	if err := validateSessionPermissionPolicy(normalized); err != nil {
		return SessionPermissionPolicy{}, err
	}
	return normalized, nil
}
