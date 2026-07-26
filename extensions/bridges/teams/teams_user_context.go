package main

import (
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func bridgeHealthMessage(degradation *bridgepkg.BridgeDegradation) string {
	if degradation == nil {
		return ""
	}
	if message := strings.TrimSpace(degradation.Message); message != "" {
		return message
	}
	return strings.TrimSpace(string(degradation.Reason))
}

func (p *teamsProvider) storeUserContext(instanceID string, activity teamsActivity) {
	userID := normalizeTeamsID(activity.From.ID)
	if userID == "" {
		return
	}
	ctx := teamsUserContext{
		ServiceURL: normalizeURL(activity.ServiceURL),
		TenantID:   strings.TrimSpace(extractTeamsTenantID(activity)),
	}
	if ctx.ServiceURL == "" && ctx.TenantID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	current := p.userContexts[userContextKey(instanceID, userID)]
	if ctx.ServiceURL == "" {
		ctx.ServiceURL = current.ServiceURL
	}
	if ctx.TenantID == "" {
		ctx.TenantID = current.TenantID
	}
	p.userContexts[userContextKey(instanceID, userID)] = ctx
}

func (p *teamsProvider) userContext(instanceID string, userID string) (teamsUserContext, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ctx, ok := p.userContexts[userContextKey(instanceID, normalizeTeamsID(userID))]
	return ctx, ok
}
