package heartbeat

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"
)

type CheckInsPolicy struct {
	AllowWrites   []string `json:"allow_writes,omitempty"`
	MaxToolCalls  *int     `json:"max_tool_calls,omitempty"`
	MaxWriteCalls *int     `json:"max_write_calls,omitempty"`
}

type AuthorityAction struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Instructions string `json:"instructions"`
	AutoAccept   bool   `json:"auto_accept"`
}

func AuthorityOverrides(servers []compozyconfig.MCPServer) map[string][]string {
	out := map[string][]string{}
	for _, server := range servers {
		if len(server.ReadOnlyTools) > 0 {
			names := slices.Clone(server.ReadOnlyTools)
			slices.Sort(names)
			out[server.EffectiveRuntimeName()] = slices.Compact(names)
		}
	}
	return out
}

func AuthorityDigest(
	policy CheckInsPolicy,
	actions []AuthorityAction,
	overrides ...map[string][]string,
) (string, error) {
	selected := make(map[string][]string)
	for _, block := range overrides {
		for server, tools := range block {
			names := slices.Clone(tools)
			slices.Sort(names)
			if len(names) > 0 {
				selected[server] = slices.Compact(names)
			}
		}
	}
	if len(policy.AllowWrites) == 0 && policy.MaxToolCalls == nil && policy.MaxWriteCalls == nil && len(actions) == 0 &&
		len(selected) == 0 {
		return "", nil
	}
	policy.AllowWrites = slices.Clone(policy.AllowWrites)
	slices.Sort(policy.AllowWrites)
	policy.AllowWrites = slices.Compact(policy.AllowWrites)
	actions = slices.Clone(actions)
	if actions == nil {
		actions = []AuthorityAction{}
	}
	slices.SortFunc(actions, func(a, b AuthorityAction) int { return strings.Compare(a.ID, b.ID) })
	data, err := json.Marshal(struct {
		CheckIns      CheckInsPolicy      `json:"check_ins"`
		Actions       []AuthorityAction   `json:"actions"`
		ReadOnlyTools map[string][]string `json:"read_only_tools,omitempty"`
	}{policy, actions, selected})
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func parseCheckIns(value any) (CheckInsPolicy, error) {
	var policy CheckInsPolicy
	if _, ok := value.(map[string]any); !ok {
		return policy, fmt.Errorf("heartbeat_check_ins_invalid: check_ins must be an object")
	}
	data, err := json.Marshal(value)
	if err != nil {
		return policy, err
	}
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&policy); err != nil {
		return policy, fmt.Errorf("heartbeat_check_ins_invalid: %w", err)
	}
	if policy.MaxToolCalls != nil && (*policy.MaxToolCalls < 1 || *policy.MaxToolCalls > 100) ||
		policy.MaxWriteCalls != nil && (*policy.MaxWriteCalls < 0 || *policy.MaxWriteCalls > 50) ||
		len(policy.AllowWrites) > 32 {
		return policy, fmt.Errorf("heartbeat_check_ins_invalid: invalid caps or too many selectors")
	}
	for _, selector := range policy.AllowWrites {
		if !validCheckInSelector(selector) {
			return policy, fmt.Errorf("heartbeat_check_ins_invalid: invalid selector %q", selector)
		}
	}
	return policy, nil
}

type CheckInLimits struct{ MaxToolCalls, MaxWriteCalls int }

func (p CheckInsPolicy) ValidateBounds(limits CheckInLimits) error {
	if p.MaxToolCalls != nil && *p.MaxToolCalls > limits.MaxToolCalls ||
		p.MaxWriteCalls != nil && *p.MaxWriteCalls > limits.MaxWriteCalls {
		return fmt.Errorf("heartbeat_check_ins_invalid: caps exceed daemon limits")
	}
	return nil
}

var checkInSelectorSegment = regexp.MustCompile(`^[a-z](?:[a-z0-9_]*[a-z0-9])?$`)

func validCheckInSelector(selector string) bool {
	if len(selector) > 64 || strings.Contains(selector, "___") {
		return false
	}
	parts := strings.Split(selector, "__")
	if len(parts) != 2 && len(parts) != 3 {
		return false
	}
	if len(parts) == 2 && parts[0] != "compozy" {
		return false
	}
	if len(parts) == 3 && parts[0] != "mcp" && parts[0] != "ext" {
		return false
	}
	for i, part := range parts {
		if part == "*" && i == 2 && parts[0] == "mcp" {
			continue
		}
		if !checkInSelectorSegment.MatchString(part) {
			return false
		}
	}
	return true
}
func CheckInSelectorWarnings(policy CheckInsPolicy, readAccess map[string]bool) []Diagnostic {
	var diagnostics []Diagnostic
	for _, selector := range policy.AllowWrites {
		known, writable := false, false
		for id, readOnly := range readAccess {
			if selector == id ||
				strings.HasSuffix(selector, "__*") && strings.HasPrefix(id, strings.TrimSuffix(selector, "*")) {
				known = true
				writable = writable || !readOnly
			}
		}
		if !known || !writable {
			diagnostics = append(
				diagnostics,
				Diagnostic{
					Code:     "heartbeat_allow_writes_unknown_tool",
					Severity: diagnosticWarning,
					Field:    "check_ins.allow_writes",
					Message:  fmt.Sprintf("%s matches no current write tool", selector),
				},
			)
		}
	}
	return diagnostics
}
