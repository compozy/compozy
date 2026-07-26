package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *gchatProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	managed, err := gchatControlInstance(session, request.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	config, configCheck := p.gchatControlConfig(session, *managed)
	checks := make([]bridgepkg.BridgeCheckRecord, 0, 3)
	if configCheck != nil {
		checks = append(checks, *configCheck)
	} else {
		checks = append(checks, gchatIdentityChecks(p.apiFactory(&config).ValidateAuth(ctx))...)
	}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func gchatControlInstance(
	session *bridgesdk.Session,
	instanceID string,
) (*subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil || session.Cache() == nil {
		return nil, errors.New("gchat: control session cache is required")
	}
	managed, ok := session.Cache().Get(instanceID)
	if !ok {
		return nil, fmt.Errorf(
			"gchat: bridge instance %q is not in the control session",
			strings.TrimSpace(instanceID),
		)
	}
	return managed, nil
}

func (p *gchatProvider) gchatControlConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (resolvedInstanceConfig, *bridgepkg.BridgeCheckRecord) {
	credentialsJSON, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "credentials_json")
	if strings.TrimSpace(credentialsJSON) == "" {
		record := bridgepkg.FailedSecretCheck("provider.identity", "credentials_json")
		return resolvedInstanceConfig{}, &record
	}
	providerConfig := gchatProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &providerConfig); err != nil {
			record := bridgepkg.BridgeCheckRecord{
				Check:       "provider.configuration",
				Status:      bridgepkg.BridgeCheckStatusFail,
				Remediation: "Fix the Google Chat provider_config JSON, then run bridge verification again.",
			}
			return resolvedInstanceConfig{}, &record
		}
	}
	credentials, err := resolveGChatCredentials(session, managed)
	if err != nil {
		record := bridgepkg.FailedSecretCheck("provider.identity", "credentials_json")
		return resolvedInstanceConfig{}, &record
	}
	config, err := p.newResolvedGChatConfig(managed, providerConfig, credentials, session)
	if err == nil {
		err = validateResolvedGChatConfig(&config, providerConfig.Mode)
	}
	if err != nil {
		record := bridgepkg.BridgeCheckRecord{
			Check:       "provider.configuration",
			Status:      bridgepkg.BridgeCheckStatusFail,
			Remediation: "Fix the Google Chat provider configuration, then run bridge verification again.",
		}
		return resolvedInstanceConfig{}, &record
	}
	return config, nil
}

func gchatIdentityChecks(err error) []bridgepkg.BridgeCheckRecord {
	return []bridgepkg.BridgeCheckRecord{
		bridgesdk.ProviderIdentityCheckRecord(err, "credentials_json"),
	}
}

func (p *gchatProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
