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

type telegramSetWebhookRequest struct {
	URL                string `json:"url"`
	SecretToken        string `json:"secret_token"`
	DropPendingUpdates bool   `json:"drop_pending_updates"`
}

type telegramWebhookAPI interface {
	SetWebhook(context.Context, telegramSetWebhookRequest) error
}

func (p *telegramProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	managed, err := telegramControlInstance(session, request.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeCheckResponse{}, err
	}
	config, configCheck := telegramControlConfig(session, *managed)
	checks := make([]bridgepkg.BridgeCheckRecord, 0, 3)
	if configCheck != nil {
		checks = append(checks, *configCheck)
	} else {
		identity, identityErr := p.apiFactory(&config).GetMe(ctx)
		checks = append(checks, telegramIdentityChecks(identity, identityErr)...)
	}
	webhookSecret, webhookSecretBound := session.Cache().BoundSecretValue(managed.Instance.ID, "webhook_secret")
	if !webhookSecretBound || strings.TrimSpace(webhookSecret) == "" {
		checks = append(checks, bridgepkg.FailedSecretCheck("webhook.secret", "webhook_secret"))
	} else {
		checks = append(checks, bridgepkg.PassCheck("webhook.secret"))
	}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func (p *telegramProvider) handleBridgeWebhookRegistration(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeWebhookRegistrationRequest,
) (bridgepkg.BridgeWebhookRegistrationResponse, error) {
	managed, err := telegramControlInstance(session, request.BridgeInstanceID)
	if err != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{}, err
	}
	if managed.Instance.Enabled {
		return bridgepkg.BridgeWebhookRegistrationResponse{
			Status:      bridgepkg.BridgeCheckStatusFail,
			Remediation: "Disable the Telegram bridge before registering a new webhook.",
		}, nil
	}

	config, configCheck := telegramControlConfig(session, *managed)
	if configCheck != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{
			Status:      configCheck.Status,
			Remediation: configCheck.Remediation,
		}, nil
	}
	publicURL, err := bridgepkg.WebhookPublicURL(managed.Instance)
	if err != nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{
			Status: bridgepkg.BridgeCheckStatusFail,
			Remediation: "Set provider_config.webhook.public_url to the external Telegram callback URL, " +
				"then register the webhook again.",
		}, nil
	}
	if strings.TrimSpace(config.webhookSecret) == "" {
		record := bridgepkg.FailedSecretCheck("webhook.registration", "webhook_secret")
		return bridgepkg.BridgeWebhookRegistrationResponse{
			Status:      record.Status,
			Remediation: record.Remediation,
		}, nil
	}
	webhookAPI, ok := p.apiFactory(&config).(telegramWebhookAPI)
	if !ok {
		return bridgepkg.BridgeWebhookRegistrationResponse{}, errors.New(
			"telegram: provider api does not implement webhook registration",
		)
	}
	err = webhookAPI.SetWebhook(ctx, telegramSetWebhookRequest{
		URL:                publicURL,
		SecretToken:        config.webhookSecret,
		DropPendingUpdates: true,
	})
	if err == nil {
		return bridgepkg.BridgeWebhookRegistrationResponse{Status: bridgepkg.BridgeCheckStatusPass}, nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return bridgepkg.BridgeWebhookRegistrationResponse{
			Status: bridgepkg.BridgeCheckStatusWarn,
			Remediation: "Telegram may have accepted the webhook before the timeout. " +
				"Run bridge verification before retrying registration.",
		}, nil
	}
	return bridgepkg.BridgeWebhookRegistrationResponse{
		Status: bridgepkg.BridgeCheckStatusFail,
		Remediation: "Verify the bot_token binding and public webhook URL, " +
			"then register the Telegram webhook again.",
	}, nil
}

func telegramControlInstance(
	session *bridgesdk.Session,
	instanceID string,
) (*subprocess.InitializeBridgeManagedInstance, error) {
	if session == nil || session.Cache() == nil {
		return nil, errors.New("telegram: control session cache is required")
	}
	managed, ok := session.Cache().Get(instanceID)
	if !ok {
		return nil, fmt.Errorf(
			"telegram: bridge instance %q is not in the control session",
			strings.TrimSpace(instanceID),
		)
	}
	return managed, nil
}

func telegramControlConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (resolvedInstanceConfig, *bridgepkg.BridgeCheckRecord) {
	providerConfig, err := decodeTelegramProviderConfig(managed)
	if err != nil {
		record := bridgepkg.BridgeCheckRecord{
			Check:       "provider.configuration",
			Status:      bridgepkg.BridgeCheckStatusFail,
			Remediation: "Fix the Telegram provider_config JSON, then run bridge verification again.",
		}
		return resolvedInstanceConfig{}, &record
	}
	config := buildTelegramResolvedInstance(session, managed, providerConfig)
	if strings.TrimSpace(config.botToken) == "" {
		record := bridgepkg.FailedSecretCheck("provider.identity", "bot_token")
		return resolvedInstanceConfig{}, &record
	}
	return config, nil
}

func telegramIdentityChecks(
	identity *telegramBotIdentity,
	err error,
) []bridgepkg.BridgeCheckRecord {
	if err != nil {
		return []bridgepkg.BridgeCheckRecord{
			bridgesdk.ProviderIdentityCheckRecord(err, "bot_token"),
		}
	}
	if identity == nil || identity.ID == 0 {
		return []bridgepkg.BridgeCheckRecord{bridgepkg.FailedSecretCheck("provider.identity", "bot_token")}
	}
	return []bridgepkg.BridgeCheckRecord{bridgesdk.ProviderIdentityCheckRecord(nil, "bot_token")}
}

func (c *telegramBotClient) SetWebhook(
	ctx context.Context,
	request telegramSetWebhookRequest,
) error {
	var accepted bool
	if err := c.callMutation(ctx, "setWebhook", request, &accepted); err != nil {
		return err
	}
	if !accepted {
		return errors.New("telegram: setWebhook was not accepted")
	}
	return nil
}

func (p *telegramProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
