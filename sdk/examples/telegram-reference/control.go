package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
)

func (r *telegramReferenceRuntime) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if session == nil || session.Cache() == nil {
		return bridgepkg.BridgeCheckResponse{}, errors.New("telegram reference: control session cache is required")
	}
	managed, ok := session.Cache().Get(request.BridgeInstanceID)
	if !ok {
		return bridgepkg.BridgeCheckResponse{}, fmt.Errorf(
			"telegram reference: bridge instance %q is not in the control session",
			strings.TrimSpace(request.BridgeInstanceID),
		)
	}
	checks := []bridgepkg.BridgeCheckRecord{referenceIdentityCheck()}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func referenceIdentityCheck() bridgepkg.BridgeCheckRecord {
	return bridgepkg.SkippedCheck(
		"provider.identity",
		"The reference adapter does not call Telegram identity APIs; use the bundled Telegram bridge to verify credentials.",
	)
}

func (r *telegramReferenceRuntime) healthCheck() error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if strings.TrimSpace(r.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(r.lastError))
}
