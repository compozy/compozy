package acp

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"

	compozyconfig "github.com/compozy/compozy/internal/config"

	"github.com/bmatcuk/doublestar"
	acpsdk "github.com/coder/acp-go-sdk"
)

func (p permissionPolicy) ruleDecision(
	request acpsdk.RequestPermissionRequest,
	paths []string,
) (permissionDecision, bool) {
	allowed := false
	for _, rule := range p.rules {
		if rule.Kind != "" && (request.ToolCall.Kind == nil || string(*request.ToolCall.Kind) != rule.Kind) {
			continue
		}
		if rule.Path != "" && !p.rulePathsMatch(rule, paths) {
			continue
		}

		if rule.Decision == "deny" {
			return decisionRejectOnce, true
		}
		allowed = true
	}
	return decisionAllowOnce, allowed
}

// An allow must cover the entire operation; any denied location vetoes it.
func (p permissionPolicy) rulePathsMatch(rule compozyconfig.PermissionRule, paths []string) bool {
	if len(paths) == 0 {
		return false
	}
	allow := rule.Decision == "allow"
	for _, location := range paths {
		relative, err := filepath.Rel(p.root, location)
		match := false
		if err == nil {
			match, err = doublestar.Match(rule.Path, filepath.ToSlash(relative))
		}
		matched := err == nil && match
		if allow && !matched {
			return false
		}
		if !allow && matched {
			return true
		}
	}
	return allow
}

type permissionDecisionSourceHost interface {
	PermissionDecisionSource(acpsdk.RequestPermissionRequest) (PermissionDecision, bool, string)
}

func resolveHostPermissionDecision(
	host ToolHost,
	request acpsdk.RequestPermissionRequest,
) (PermissionDecision, bool, string) {
	if sourced, ok := host.(permissionDecisionSourceHost); ok {
		return sourced.PermissionDecisionSource(request)
	}
	decision, interactive := host.PermissionDecision(request)
	return decision, interactive, permissionSourcePolicy
}

func (p *AgentProcess) automaticPermissionOutcome(requestID string, request acpsdk.RequestPermissionRequest,
	decision permissionDecision, source string) (acpsdk.RequestPermissionOutcome, permissionDecision, json.RawMessage) {
	options := request.Options
	if source == permissionSourceRule {
		// A rule grants or rejects one operation, never a provider-wide remembered grant.
		options = slices.DeleteFunc(slices.Clone(options), func(option acpsdk.PermissionOption) bool {
			return option.Kind == acpsdk.PermissionOptionKindAllowAlways ||
				option.Kind == acpsdk.PermissionOptionKindRejectAlways
		})
	}
	outcome, applied := selectPermissionOutcome(options, decision)
	if applied == "" {
		applied = decisionCanceled
	}
	raw := buildPermissionEventRaw(requestID, applied, request)
	if source == permissionSourceRule {
		paths, err := p.permissions.resolvePathList(request.ToolCall.Locations)
		if err == nil {
			for index, rule := range p.permissions.rules {
				if rule.Kind != "" && (request.ToolCall.Kind == nil || string(*request.ToolCall.Kind) != rule.Kind) {
					continue
				}
				if rule.Path != "" && !p.permissions.rulePathsMatch(rule, paths) {
					continue
				}
				if (rule.Decision == "deny") != (decision == decisionRejectOnce) {
					continue
				}
				var payload map[string]any
				if json.Unmarshal(raw, &payload) != nil {
					break
				}
				payload["permission_rule"] = map[string]any{
					"index":    index,
					"kind":     rule.Kind,
					"path":     rule.Path,
					"decision": rule.Decision,
				}
				if encoded, err := json.Marshal(payload); err == nil {
					raw = encoded
				}
				break
			}
		}
	}
	return outcome, applied, raw
}

func (p *AgentProcess) permissionModeDegraded(caps Caps) bool {
	if p.permissions.mode != compozyconfig.PermissionModeAuto {
		return false
	}
	if p.reviewerMode == "" || compozyconfig.IsUnrestrictedACPMode(p.permissionModeProvider(), p.reviewerMode) {
		return true
	}
	for _, option := range caps.ConfigOptions {
		if option.ID == sessionConfigModeKey {
			return strings.TrimSpace(option.CurrentValueID) != p.reviewerMode
		}
	}
	return true
}

func (p *AgentProcess) permissionModeProvider() string {
	if p.modeProviderName != "" {
		return p.modeProviderName
	}
	return p.providerName
}
