package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *discordProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	managed, err := discordControlInstance(session, request.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	config, configCheck := discordControlConfig(session, *managed)
	checks := make([]bridgepkg.BridgeCheckRecord, 0, 3)
	if configCheck != nil {
		checks = append(checks, *configCheck)
	} else {
		identity, identityErr := p.apiFactory(config).GetBotUser(ctx)
		checks = append(checks, discordIdentityChecks(identity, config.applicationID, identityErr)...)
	}
	publicKey, publicKeyBound := session.Cache().BoundSecretValue(managed.Instance.ID, "public_key")
	if !publicKeyBound || strings.TrimSpace(publicKey) == "" {
		checks = append(checks, bridgepkg.FailedSecretCheck("webhook.public_key", "public_key"))
	} else {
		checks = append(checks, bridgepkg.PassCheck("webhook.public_key"))
	}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func discordControlInstance(
	session *bridgesdk.Session,
	instanceID string,
) (*subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil || session.Cache() == nil {
		return nil, errors.New("discord: control session cache is required")
	}
	managed, ok := session.Cache().Get(instanceID)
	if !ok {
		return nil, fmt.Errorf(
			"discord: bridge instance %q is not in the control session",
			strings.TrimSpace(instanceID),
		)
	}
	return managed, nil
}

func discordControlConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (resolvedInstanceConfig, *bridgepkg.BridgeCheckRecord) {
	providerConfig := discordProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &providerConfig); err != nil {
			record := bridgepkg.BridgeCheckRecord{
				Check:       "provider.configuration",
				Status:      bridgepkg.BridgeCheckStatusFail,
				Remediation: "Fix the Discord provider_config JSON, then run bridge verification again.",
			}
			return resolvedInstanceConfig{}, &record
		}
	}
	botToken, _ := session.Cache().BoundSecretValue(managed.Instance.ID, "bot_token")
	if strings.TrimSpace(botToken) == "" {
		record := bridgepkg.FailedSecretCheck("provider.identity", "bot_token")
		return resolvedInstanceConfig{}, &record
	}
	return resolvedInstanceConfig{
		managed:    managed,
		instanceID: strings.TrimSpace(managed.Instance.ID),
		apiBaseURL: normalizeURL(firstNonEmpty(
			strings.TrimSpace(os.Getenv(discordAPIBaseEnv)),
			discordDefaultAPIBaseURL,
		)),
		applicationID: strings.TrimSpace(providerConfig.ApplicationID),
		botToken:      strings.TrimSpace(botToken),
	}, nil
}

func discordIdentityChecks(
	identity *discordBotIdentity,
	applicationID string,
	err error,
) []bridgepkg.BridgeCheckRecord {
	if err != nil {
		return []bridgepkg.BridgeCheckRecord{
			bridgesdk.ProviderIdentityCheckRecord(err, "bot_token"),
		}
	}
	if identity == nil || strings.TrimSpace(identity.ID) == "" {
		return []bridgepkg.BridgeCheckRecord{bridgepkg.FailedSecretCheck("provider.identity", "bot_token")}
	}
	configuredID := strings.TrimSpace(applicationID)
	if configuredID != "" && strings.TrimSpace(identity.ID) != configuredID {
		return []bridgepkg.BridgeCheckRecord{{
			Check:  "provider.identity",
			Status: bridgepkg.BridgeCheckStatusFail,
			Remediation: "Set provider_config.application_id to the authenticated Discord bot ID, " +
				"then run bridge verification again.",
		}}
	}
	return []bridgepkg.BridgeCheckRecord{bridgesdk.ProviderIdentityCheckRecord(nil, "bot_token")}
}

func (p *discordProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
