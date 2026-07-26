package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
	"github.com/compozy/agh/internal/bridgesdk"
	"github.com/compozy/agh/internal/subprocess"
)

func (p *slackProvider) handleBridgeCheck(
	ctx context.Context,
	session *bridgesdk.Session,
	request bridgepkg.BridgeCheckRequest,
) (bridgepkg.BridgeCheckResponse, error) {
	if session == nil || session.Cache() == nil {
		return bridgepkg.BridgeCheckResponse{}, errors.New("slack: control session cache is required")
	}
	managed, ok := session.Cache().Get(request.BridgeInstanceID)
	if !ok {
		return bridgepkg.BridgeCheckResponse{}, fmt.Errorf(
			"slack: bridge instance %q is not in the control session",
			strings.TrimSpace(request.BridgeInstanceID),
		)
	}

	checks := make([]bridgepkg.BridgeCheckRecord, 0, len(bridgepkg.SlackBotScopes())+3)
	config, configCheck := p.slackControlConfig(session, *managed)
	if configCheck != nil {
		checks = append(checks, *configCheck)
	} else {
		identity, err := p.apiFactory(&config).AuthTest(ctx)
		checks = append(checks, slackIdentityChecks(identity, err)...)
	}
	signingSecret, signingSecretBound := session.Cache().BoundSecretValue(
		managed.Instance.ID,
		"signing_secret",
	)
	if !signingSecretBound || strings.TrimSpace(signingSecret) == "" {
		checks = append(checks, bridgepkg.FailedSecretCheck("webhook.signing_secret", "signing_secret"))
	} else {
		checks = append(checks, bridgepkg.PassCheck("webhook.signing_secret"))
	}
	checks = append(checks, bridgesdk.WebhookCheckRecords(ctx, managed.Instance, nil)...)
	return bridgepkg.BridgeCheckResponse{Checks: checks}, nil
}

func (p *slackProvider) slackControlConfig(
	session *bridgesdk.Session,
	managed subprocess.InitializeBridgeManagedInstance,
) (resolvedInstanceConfig, *bridgepkg.BridgeCheckRecord) {
	providerConfig := slackProviderConfig{}
	if len(managed.Instance.ProviderConfig) > 0 {
		if err := json.Unmarshal(managed.Instance.ProviderConfig, &providerConfig); err != nil {
			record := bridgepkg.BridgeCheckRecord{
				Check:       "provider.configuration",
				Status:      bridgepkg.BridgeCheckStatusFail,
				Remediation: "Fix the Slack provider_config JSON, then run bridge verification again.",
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
		managed:    &managed,
		instanceID: strings.TrimSpace(managed.Instance.ID),
		apiBaseURL: normalizeURL(firstNonEmpty(
			strings.TrimSpace(os.Getenv(slackAPIBaseEnv)),
			slackDefaultAPIBaseURL,
		)),
		botToken: strings.TrimSpace(botToken),
	}, nil
}

func slackIdentityChecks(identity *slackAuthIdentity, err error) []bridgepkg.BridgeCheckRecord {
	if err != nil {
		return []bridgepkg.BridgeCheckRecord{
			bridgesdk.ProviderIdentityCheckRecord(err, "bot_token"),
		}
	}
	if identity == nil {
		return []bridgepkg.BridgeCheckRecord{bridgepkg.FailedSecretCheck("provider.identity", "bot_token")}
	}
	if strings.TrimSpace(identity.BotID) == "" {
		return []bridgepkg.BridgeCheckRecord{{
			Check:  "provider.identity",
			Status: bridgepkg.BridgeCheckStatusFail,
			Remediation: "The bot_token binding contains a user token, expected bot token. " +
				"Reinstall the Slack app and bind its Bot User OAuth token.",
		}}
	}

	checks := []bridgepkg.BridgeCheckRecord{bridgesdk.ProviderIdentityCheckRecord(nil, "bot_token")}
	granted := make(map[string]struct{}, len(identity.GrantedScopes))
	for _, scope := range identity.GrantedScopes {
		granted[strings.TrimSpace(scope)] = struct{}{}
	}
	for _, scope := range bridgepkg.SlackBotScopes() {
		if _, ok := granted[scope]; ok {
			checks = append(checks, bridgepkg.PassCheck("scope."+scope))
			continue
		}
		checks = append(checks, bridgepkg.BridgeCheckRecord{
			Check:  "scope." + scope,
			Status: bridgepkg.BridgeCheckStatusFail,
			Remediation: fmt.Sprintf(
				"Add the %s bot scope, reinstall the Slack app, then run bridge verification again.",
				scope,
			),
		})
	}
	return checks
}

func splitSlackScopes(value string) []string {
	seen := make(map[string]struct{})
	for raw := range strings.SplitSeq(value, ",") {
		if scope := strings.TrimSpace(raw); scope != "" {
			seen[scope] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for scope := range seen {
		result = append(result, scope)
	}
	sort.Strings(result)
	return result
}

func (p *slackProvider) healthCheck() error {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if strings.TrimSpace(p.lastError) == "" {
		return nil
	}
	return errors.New(strings.TrimSpace(p.lastError))
}
