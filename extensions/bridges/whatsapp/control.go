package main

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *whatsappProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	managed, err := whatsappControlInstance(session, request.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	config, configCheck := whatsappControlConfig(session, *managed)
	checks := make([]bridgepkg.BridgeCheckRecord, 0, 5)
	switch {
	case configCheck != nil:
		checks = append(checks, *configCheck)
	case strings.TrimSpace(config.accessToken) == "":
		checks = append(checks, bridgepkg.FailedSecretCheck("provider.identity", "access_token"))
	default:
		identity, identityErr := p.apiFactory(config).GetPhoneNumber(ctx, config.phoneNumberID)
		checks = append(checks, whatsappIdentityChecks(identity, identityErr)...)
	}
	checks = append(checks, whatsappRequiredSecretChecks(config)...)
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func whatsappControlInstance(
	session *bridgesdk.Session,
	instanceID string,
) (*subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil || session.Cache() == nil {
		return nil, errors.New("whatsapp: control session cache is required")
	}
	managed, ok := session.Cache().Get(instanceID)
	if !ok {
		return nil, fmt.Errorf(
			"whatsapp: bridge instance %q is not in the control session",
			strings.TrimSpace(instanceID),
		)
	}
	return managed, nil
}

func whatsappControlConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (resolvedInstanceConfig, *bridgepkg.BridgeCheckRecord) {
	providerConfig, err := decodeWhatsAppProviderConfig(managed)
	if err != nil {
		record := bridgepkg.BridgeCheckRecord{
			Check:       "provider.configuration",
			Status:      bridgepkg.BridgeCheckStatusFail,
			Remediation: "Fix the WhatsApp provider_config JSON, then run bridge verification again.",
		}
		return resolvedInstanceConfig{}, &record
	}
	config := buildWhatsAppResolvedInstance(session, managed, providerConfig)
	validateWhatsAppResolvedConfig(&config)
	if config.configError != nil {
		record := bridgepkg.BridgeCheckRecord{
			Check:  "provider.configuration",
			Status: bridgepkg.BridgeCheckStatusFail,
			Remediation: "Set provider_config.phone_number_id to the 15–17 digit Meta ID, " +
				"then run bridge verification again.",
		}
		return resolvedInstanceConfig{}, &record
	}
	return config, nil
}

func whatsappIdentityChecks(
	identity *whatsappPhoneNumber,
	err error,
) []bridgepkg.BridgeCheckRecord {
	if err != nil {
		return []bridgepkg.BridgeCheckRecord{
			bridgesdk.ProviderIdentityCheckRecord(err, "access_token"),
		}
	}
	if identity == nil || strings.TrimSpace(identity.ID) == "" {
		return []bridgepkg.BridgeCheckRecord{bridgepkg.FailedSecretCheck("provider.identity", "access_token")}
	}
	return []bridgepkg.BridgeCheckRecord{bridgesdk.ProviderIdentityCheckRecord(nil, "access_token")}
}

func whatsappRequiredSecretChecks(config resolvedInstanceConfig) []bridgepkg.BridgeCheckRecord {
	checks := make([]bridgepkg.BridgeCheckRecord, 0, 2)
	for _, secret := range []struct {
		check string
		name  string
		value string
	}{
		{check: "webhook.app_secret", name: whatsappAppSecretSlot, value: config.appSecret},
		{check: "webhook.verify_token", name: whatsappVerifyTokenSlot, value: config.verifyToken},
	} {
		if strings.TrimSpace(secret.value) == "" {
			checks = append(checks, bridgepkg.FailedSecretCheck(secret.check, secret.name))
			continue
		}
		checks = append(checks, bridgepkg.PassCheck(secret.check))
	}
	return checks
}

func (p *whatsappProvider) healthCheck() error {
	p.mu.RLock()
	globalError := strings.TrimSpace(p.lastError)
	instanceErrors := make(map[string]string, len(p.instanceErrors))
	for instanceID, message := range p.instanceErrors {
		if strings.TrimSpace(message) == "" {
			continue
		}
		instanceErrors[instanceID] = strings.TrimSpace(message)
	}
	p.mu.RUnlock()
	if globalError != "" {
		return errors.New(globalError)
	}
	if len(instanceErrors) == 0 {
		return nil
	}
	instanceIDs := make([]string, 0, len(instanceErrors))
	for instanceID := range instanceErrors {
		instanceIDs = append(instanceIDs, instanceID)
	}
	sort.Strings(instanceIDs)
	return fmt.Errorf("%s: %s", instanceIDs[0], instanceErrors[instanceIDs[0]])
}
