package session

import (
	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
)

func storedPermissionRules(rules []compozyconfig.PermissionRule) []store.SessionPermissionRule {
	if rules == nil {
		return nil
	}
	result := make([]store.SessionPermissionRule, len(rules))
	for i, rule := range rules {
		result[i] = store.SessionPermissionRule{Kind: rule.Kind, Path: rule.Path, Decision: rule.Decision}
	}
	return result
}

func configuredPermissionRules(rules []store.SessionPermissionRule) []compozyconfig.PermissionRule {
	if rules == nil {
		return nil
	}
	result := make([]compozyconfig.PermissionRule, len(rules))
	for i, rule := range rules {
		result[i] = compozyconfig.PermissionRule{Kind: rule.Kind, Path: rule.Path, Decision: rule.Decision}
	}
	return result
}

func (s *sessionStartSpec) resolveDefinitionPermissionPolicy(agent compozyconfig.AgentDef) {
	if s.startAction != sessionStartActionCreate {
		return
	}
	s.effectiveWorkspaceAccess = s.origin.Effective(agent.WorkspaceAccess)
	s.effectiveSessionApproval = agent.SessionApproval
	if s.effectiveSessionApproval == "" {
		s.effectiveSessionApproval = "children"
	}
	s.permissionRules = storedPermissionRules(agent.PermissionRules)
}

func reportedPermissionMode(configured string, degraded bool) string {
	if configured == string(compozyconfig.PermissionModeAuto) && degraded {
		return string(compozyconfig.PermissionModeApproveEdits)
	}
	return configured
}

func (s *Session) permissionDenialsState() *acp.PermissionDenials {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.permissionDenials == nil {
		s.permissionDenials = &acp.PermissionDenials{}
	}
	return s.permissionDenials
}
