package acp

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	"path/filepath"
	"strings"

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
		if rule.Path != "" {
			matches := false
			for _, location := range paths {
				relative, err := filepath.Rel(p.root, location)
				if err != nil {
					continue
				}
				match, err := doublestar.Match(rule.Path, filepath.ToSlash(relative))
				if err == nil && match {
					matches = true
					break
				}
			}
			if !matches {
				continue
			}
		}
		if rule.Decision == "deny" {
			return decisionRejectOnce, true
		}
		allowed = true
	}
	return decisionAllowOnce, allowed
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
	return decision, interactive, "policy"
}

func (p *AgentProcess) permissionModeDegraded(caps Caps) bool {
	if p.permissions.mode != compozyconfig.PermissionModeAuto {
		return false
	}
	if p.reviewerMode == "" || compozyconfig.IsUnrestrictedACPMode(p.providerName, p.reviewerMode) {
		return true
	}
	for _, option := range caps.ConfigOptions {
		if option.ID == sessionConfigModeKey {
			return strings.TrimSpace(option.CurrentValueID) != p.reviewerMode
		}
	}
	return true
}
