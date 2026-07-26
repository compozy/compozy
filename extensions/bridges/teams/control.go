package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *teamsProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	managed, err := teamsControlInstance(session, request.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	config, configCheck := teamsControlConfig(session, *managed)
	checks := make([]bridgepkg.BridgeCheckRecord, 0, 3)
	if configCheck != nil {
		checks = append(checks, *configCheck)
	} else {
		checks = append(checks, teamsIdentityChecks(p.apiFactory(config).ValidateAuth(ctx))...)
	}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func teamsControlInstance(
	session *bridgesdk.Session,
	instanceID string,
) (*subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil || session.Cache() == nil {
		return nil, errors.New("teams: control session cache is required")
	}
	managed, ok := session.Cache().Get(instanceID)
	if !ok {
		return nil, fmt.Errorf(
			"teams: bridge instance %q is not in the control session",
			strings.TrimSpace(instanceID),
		)
	}
	return managed, nil
}

func teamsControlConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (resolvedInstanceConfig, *bridgepkg.BridgeCheckRecord) {
	providerConfig, err := decodeTeamsProviderConfig(managed)
	if err != nil {
		record := bridgepkg.BridgeCheckRecord{
			Check:       "provider.configuration",
			Status:      bridgepkg.BridgeCheckStatusFail,
			Remediation: "Fix the Teams provider_config JSON, then run bridge verification again.",
		}
		return resolvedInstanceConfig{}, &record
	}
	config := buildTeamsResolvedInstance(session, managed, providerConfig)
	validateTeamsResolvedConfig(&config)
	if config.configError != nil {
		record := bridgepkg.BridgeCheckRecord{
			Check:       "provider.configuration",
			Status:      bridgepkg.BridgeCheckStatusFail,
			Remediation: "Fix the Teams provider configuration, then run bridge verification again.",
		}
		return resolvedInstanceConfig{}, &record
	}
	if strings.TrimSpace(config.appID) == "" {
		record := bridgepkg.FailedSecretCheck("provider.identity", "app_id")
		return resolvedInstanceConfig{}, &record
	}
	if strings.TrimSpace(config.appPassword) == "" {
		record := bridgepkg.FailedSecretCheck("provider.identity", "app_password")
		return resolvedInstanceConfig{}, &record
	}
	return config, nil
}

func teamsIdentityChecks(err error) []bridgepkg.BridgeCheckRecord {
	return []bridgepkg.BridgeCheckRecord{
		bridgesdk.ProviderIdentityCheckRecord(err, "app_id", "app_password"),
	}
}

func (p *teamsProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for instanceID, status := range p.lifecycle.Host().Statuses() {
		normalized := status.Normalize()
		if normalized == "" || normalized == bridgepkg.BridgeStatusReady {
			continue
		}
		message := strings.TrimSpace(p.reportedHealth[instanceID])
		if message != "" {
			return fmt.Errorf("teams: bridge instance %s is %s: %s", instanceID, normalized, message)
		}
		return fmt.Errorf("teams: bridge instance %s is %s", instanceID, normalized)
	}
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
