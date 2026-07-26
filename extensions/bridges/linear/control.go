package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (p *linearProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if session == nil || session.Cache() == nil {
		return bridgepkg.BridgeCheckResponse{}, errors.New("linear: control session cache is required")
	}
	managed, ok := session.Cache().Get(request.BridgeInstanceID)
	if !ok {
		return bridgepkg.BridgeCheckResponse{}, fmt.Errorf(
			"linear: bridge instance %q is not in the control session",
			strings.TrimSpace(request.BridgeInstanceID),
		)
	}
	checks := []bridgepkg.BridgeCheckRecord{linearIdentityCheck()}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func linearIdentityCheck() bridgepkg.BridgeCheckRecord {
	return bridgepkg.SkippedCheck(
		"provider.identity",
		"Linear identity probing is not supported by this bridge control contract; "+
			"enable the bridge and inspect its runtime health.",
	)
}

func (p *linearProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
