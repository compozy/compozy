package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *githubProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if session == nil || session.Cache() == nil {
		return bridgepkg.BridgeCheckResponse{}, errors.New("github: control session cache is required")
	}
	managed, ok := session.Cache().Get(request.BridgeInstanceID)
	if !ok {
		return bridgepkg.BridgeCheckResponse{}, fmt.Errorf(
			"github: bridge instance %q is not in the control session",
			strings.TrimSpace(request.BridgeInstanceID),
		)
	}
	checks := []bridgepkg.BridgeCheckRecord{githubIdentityCheck()}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func githubIdentityCheck() bridgepkg.BridgeCheckRecord {
	return bridgepkg.SkippedCheck(
		"provider.identity",
		"GitHub identity probing is not supported by this bridge control contract; "+
			"enable the bridge and inspect its runtime health.",
	)
}

func (p *githubProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
