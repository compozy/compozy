package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

type bridgeSetupJSONInput struct {
	InstanceID        string                   `json:"instance_id,omitempty"`
	Scope             string                   `json:"scope,omitempty"`
	WorkspaceID       string                   `json:"workspace_id,omitempty"`
	DisplayName       string                   `json:"display_name,omitempty"`
	ExtensionName     string                   `json:"extension_name,omitempty"`
	RoutingPolicy     *bridgepkg.RoutingPolicy `json:"routing_policy,omitempty"`
	ProviderConfig    json.RawMessage          `json:"provider_config,omitempty"`
	WebhookPublicURL  string                   `json:"webhook_public_url,omitempty"`
	WebhookPath       string                   `json:"webhook_path,omitempty"`
	PhoneNumberID     string                   `json:"phone_number_id,omitempty"`
	AccessToken       string                   `json:"access_token,omitempty"`
	AppSecret         string                   `json:"app_secret,omitempty"`
	VerifyToken       string                   `json:"verify_token,omitempty"`
	BotToken          string                   `json:"bot_token,omitempty"`
	WebhookSecret     string                   `json:"webhook_secret,omitempty"`
	ApplicationID     string                   `json:"application_id,omitempty"`
	PublicKey         string                   `json:"public_key,omitempty"`
	InviteScopes      []string                 `json:"invite_scopes,omitempty"`
	InvitePermissions uint64                   `json:"invite_permissions,omitempty"`
}

type bridgeSetupProviderDetails struct {
	ProviderConfig json.RawMessage
	PublicURL      string
	WebhookPath    string
	InviteURL      string
}

func bridgeSetupInputFromWizard(
	descriptor bridgeSetupDescriptor,
	selection bridgeSetupWizardSelection,
	existing *BridgeRecord,
) (bridgeSetupJSONInput, error) {
	values := selection.Values
	input := bridgeSetupJSONInput{
		Scope:            values[bridgeSetupScopeKey],
		WorkspaceID:      values[bridgeSetupWorkspaceIDKey],
		DisplayName:      values[bridgeSetupDisplayNameKey],
		ExtensionName:    values[bridgeSetupExtensionNameKey],
		WebhookPublicURL: values[bridgeSetupWebhookPublicURLKey],
		WebhookPath:      values[bridgeSetupWebhookPathKey],
		PhoneNumberID:    values[bridgeSetupPhoneNumberIDKey],
		AccessToken:      values[bridgeSetupAccessTokenKey],
		AppSecret:        values[bridgeSetupAppSecretKey],
		VerifyToken:      values[bridgeSetupVerifyTokenKey],
		BotToken:         values[bridgeSetupBotTokenKey],
		WebhookSecret:    values[bridgeSetupWebhookSecretKey],
		ApplicationID:    values[bridgeSetupApplicationIDKey],
		PublicKey:        values[bridgeSetupPublicKeyKey],
		InviteScopes: normalizeDiscordInviteScopes(
			[]string{values[bridgeSetupInviteScopesKey]},
		),
	}
	if descriptor.Platform == bridgeSetupPlatformTelegram {
		routingPolicy, err := telegramRoutingPolicyForShape(values[bridgeSetupRoutingShapeKey])
		if err != nil {
			return bridgeSetupJSONInput{}, err
		}
		input.RoutingPolicy = &routingPolicy
	}
	if existing != nil {
		input.InstanceID = existing.ID
		input.ProviderConfig = append(json.RawMessage(nil), existing.ProviderConfig...)
	}
	if rawPermissions := strings.TrimSpace(values[bridgeSetupInvitePermissionsKey]); rawPermissions != "" {
		permissions, err := strconv.ParseUint(rawPermissions, 10, 64)
		if err != nil {
			return bridgeSetupJSONInput{}, fmt.Errorf("cli: parse Discord permission bits: %w", err)
		}
		input.InvitePermissions = permissions
	}
	if descriptor.Platform != bridgeSetupPlatformDiscord {
		input.InviteScopes = nil
		input.InvitePermissions = 0
	}
	return input, nil
}

func bridgeSetupWizardDefaults(
	descriptor bridgeSetupDescriptor,
	existing *BridgeRecord,
	bindings []BridgeSecretBindingRecord,
) (map[string]string, error) {
	values := map[string]string{
		bridgeSetupScopeKey:             string(bridgepkg.ScopeGlobal),
		bridgeSetupDisplayNameKey:       descriptor.DisplayName,
		bridgeSetupExtensionNameKey:     descriptor.ExtensionName,
		bridgeSetupInviteScopesKey:      "bot, applications.commands",
		bridgeSetupInvitePermissionsKey: strconv.FormatUint(bridgeSetupDiscordDefaultPermissions, 10),
	}
	if descriptor.Platform == bridgeSetupPlatformTelegram {
		values[bridgeSetupRoutingShapeKey] = bridgeSetupTelegramRoutingShapeForum
	}
	if existing != nil {
		values[bridgeSetupScopeKey] = string(existing.Scope.Normalize())
		values[bridgeSetupWorkspaceIDKey] = strings.TrimSpace(existing.WorkspaceID)
		values[bridgeSetupDisplayNameKey] = strings.TrimSpace(existing.DisplayName)
		values[bridgeSetupExtensionNameKey] = strings.TrimSpace(existing.ExtensionName)
		config, err := decodeBridgeSetupProviderConfig(existing.ProviderConfig)
		if err != nil {
			return nil, err
		}
		values[bridgeSetupWebhookPublicURLKey] = providerConfigString(config, bridgeSetupWebhookConfigKey, "public_url")
		values[bridgeSetupWebhookPathKey] = providerConfigString(config, bridgeSetupWebhookConfigKey, "path")
		values[bridgeSetupPhoneNumberIDKey] = providerConfigString(config, "phone_number_id")
		values[bridgeSetupApplicationIDKey] = providerConfigString(config, "application_id")
		if descriptor.Platform == bridgeSetupPlatformTelegram {
			routingShape, shapeErr := telegramRoutingShapeForPolicy(existing.RoutingPolicy)
			if shapeErr != nil {
				return nil, shapeErr
			}
			values[bridgeSetupRoutingShapeKey] = routingShape
		}
		if scopes := providerConfigStrings(config, "invite", "scopes"); len(scopes) > 0 {
			values[bridgeSetupInviteScopesKey] = strings.Join(scopes, ", ")
		}
		if permissions := providerConfigUint64(config, "invite", "permissions"); permissions > 0 {
			values[bridgeSetupInvitePermissionsKey] = strconv.FormatUint(permissions, 10)
		}
	}
	for _, binding := range bindings {
		name := strings.TrimSpace(binding.BindingName)
		if name != "" {
			values[name] = bridgeSetupMaskedSecret
		}
	}
	return values, nil
}

func buildBridgeSetupProviderDetails(
	platform string,
	input bridgeSetupJSONInput,
	existing *BridgeRecord,
) (bridgeSetupProviderDetails, error) {
	raw := bytes.TrimSpace(input.ProviderConfig)
	if len(raw) == 0 && existing != nil {
		raw = bytes.TrimSpace(existing.ProviderConfig)
	}
	config, err := decodeBridgeSetupProviderConfig(raw)
	if err != nil {
		return bridgeSetupProviderDetails{}, err
	}

	publicURL := firstBridgeSetupValue(
		input.WebhookPublicURL,
		providerConfigString(config, bridgeSetupWebhookConfigKey, "public_url"),
	)
	if err := validateBridgeSetupWebhookPublicURL(publicURL); err != nil {
		return bridgeSetupProviderDetails{}, fmt.Errorf("cli: invalid public webhook URL: %w", err)
	}
	webhookPath := firstBridgeSetupValue(
		input.WebhookPath,
		providerConfigString(config, bridgeSetupWebhookConfigKey, "path"),
	)
	if err := validateBridgeSetupWebhookPath(webhookPath); err != nil {
		return bridgeSetupProviderDetails{}, fmt.Errorf("cli: invalid internal webhook path: %w", err)
	}
	if err := setProviderConfigString(config, publicURL, bridgeSetupWebhookConfigKey, "public_url"); err != nil {
		return bridgeSetupProviderDetails{}, err
	}
	if err := setProviderConfigString(config, webhookPath, bridgeSetupWebhookConfigKey, "path"); err != nil {
		return bridgeSetupProviderDetails{}, err
	}

	inviteURL, err := configureBridgeSetupPlatform(platform, input, config)
	if err != nil {
		return bridgeSetupProviderDetails{}, err
	}
	details := bridgeSetupProviderDetails{
		PublicURL:   publicURL,
		WebhookPath: webhookPath,
		InviteURL:   inviteURL,
	}
	details.ProviderConfig, err = json.Marshal(config)
	if err != nil {
		return bridgeSetupProviderDetails{}, fmt.Errorf("cli: encode %s provider config: %w", platform, err)
	}
	if existing != nil && !bridgeSetupJSONEqual(existing.ProviderConfig, details.ProviderConfig) {
		return bridgeSetupProviderDetails{}, fmt.Errorf(
			"cli: bridge %q provider config differs; update the bridge before re-running setup",
			existing.ID,
		)
	}
	return details, nil
}

func configureBridgeSetupPlatform(
	platform string,
	input bridgeSetupJSONInput,
	config map[string]json.RawMessage,
) (string, error) {
	switch platform {
	case bridgeSetupPlatformWhatsApp:
		return "", configureWhatsAppBridgeSetup(input, config)
	case bridgeSetupPlatformTelegram:
		return "", nil
	case bridgeSetupPlatformDiscord:
		return configureDiscordBridgeSetup(input, config)
	default:
		return "", fmt.Errorf("cli: unsupported bridge setup platform %q", platform)
	}
}

func configureWhatsAppBridgeSetup(
	input bridgeSetupJSONInput,
	config map[string]json.RawMessage,
) error {
	phoneNumberID := firstBridgeSetupValue(
		input.PhoneNumberID,
		providerConfigString(config, "phone_number_id"),
	)
	if err := validateWhatsAppPhoneNumberID(phoneNumberID); err != nil {
		return fmt.Errorf("cli: invalid WhatsApp phone_number_id: %w", err)
	}
	if err := setProviderConfigString(config, phoneNumberID, "phone_number_id"); err != nil {
		return err
	}
	return nil
}

func configureDiscordBridgeSetup(
	input bridgeSetupJSONInput,
	config map[string]json.RawMessage,
) (string, error) {
	applicationID := firstBridgeSetupValue(
		input.ApplicationID,
		providerConfigString(config, "application_id"),
	)
	if err := validateDiscordApplicationID(applicationID); err != nil {
		return "", fmt.Errorf("cli: invalid Discord application_id: %w", err)
	}
	if err := setProviderConfigString(config, applicationID, "application_id"); err != nil {
		return "", err
	}
	scopes := input.InviteScopes
	if len(scopes) == 0 {
		scopes = providerConfigStrings(config, "invite", "scopes")
	}
	scopes = normalizeDiscordInviteScopes(scopes)
	permissions := input.InvitePermissions
	if permissions == 0 {
		permissions = providerConfigUint64(config, "invite", "permissions")
	}
	if permissions == 0 {
		permissions = bridgeSetupDiscordDefaultPermissions
	}
	if err := setProviderConfigValue(config, scopes, "invite", "scopes"); err != nil {
		return "", err
	}
	if err := setProviderConfigValue(config, permissions, "invite", "permissions"); err != nil {
		return "", err
	}
	return buildDiscordInviteURL(applicationID, scopes, permissions)
}

func bridgeSetupSecretValues(input bridgeSetupJSONInput) map[string]string {
	return map[string]string{
		bridgeSetupAccessTokenKey:   strings.TrimSpace(input.AccessToken),
		bridgeSetupAppSecretKey:     strings.TrimSpace(input.AppSecret),
		bridgeSetupVerifyTokenKey:   strings.TrimSpace(input.VerifyToken),
		bridgeSetupBotTokenKey:      strings.TrimSpace(input.BotToken),
		bridgeSetupWebhookSecretKey: strings.TrimSpace(input.WebhookSecret),
		bridgeSetupPublicKeyKey:     strings.TrimSpace(input.PublicKey),
	}
}

func decodeBridgeSetupProviderConfig(raw json.RawMessage) (map[string]json.RawMessage, error) {
	if len(bytes.TrimSpace(raw)) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return make(map[string]json.RawMessage), nil
	}
	var config map[string]json.RawMessage
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, fmt.Errorf("cli: decode bridge provider config: %w", err)
	}
	if config == nil {
		return nil, errors.New("cli: bridge provider config must be a JSON object")
	}
	return config, nil
}

func setProviderConfigString(config map[string]json.RawMessage, value string, path ...string) error {
	return setProviderConfigValue(config, strings.TrimSpace(value), path...)
}

func setProviderConfigValue(config map[string]json.RawMessage, value any, path ...string) error {
	if len(path) == 0 {
		return errors.New("cli: provider config path is required")
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("cli: encode provider config %s: %w", strings.Join(path, "."), err)
	}
	if len(path) == 1 {
		config[path[0]] = encoded
		return nil
	}
	child, err := decodeBridgeSetupProviderConfig(config[path[0]])
	if err != nil {
		return fmt.Errorf("cli: decode provider config %s: %w", path[0], err)
	}
	if err := setProviderConfigValue(child, value, path[1:]...); err != nil {
		return err
	}
	config[path[0]], err = json.Marshal(child)
	if err != nil {
		return fmt.Errorf("cli: encode provider config %s: %w", path[0], err)
	}
	return nil
}

func providerConfigString(config map[string]json.RawMessage, path ...string) string {
	raw, ok := providerConfigRaw(config, path...)
	if !ok {
		return ""
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func providerConfigStrings(config map[string]json.RawMessage, path ...string) []string {
	raw, ok := providerConfigRaw(config, path...)
	if !ok {
		return nil
	}
	var values []string
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil
	}
	return values
}

func providerConfigUint64(config map[string]json.RawMessage, path ...string) uint64 {
	raw, ok := providerConfigRaw(config, path...)
	if !ok {
		return 0
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return 0
	}
	return value
}

func providerConfigRaw(config map[string]json.RawMessage, path ...string) (json.RawMessage, bool) {
	if len(path) == 0 {
		return nil, false
	}
	raw, ok := config[path[0]]
	if !ok || len(path) == 1 {
		return raw, ok
	}
	child, err := decodeBridgeSetupProviderConfig(raw)
	if err != nil {
		return nil, false
	}
	return providerConfigRaw(child, path[1:]...)
}

func bridgeSetupJSONEqual(left json.RawMessage, right json.RawMessage) bool {
	leftValue, err := decodeBridgeSetupComparableJSON(left)
	if err != nil {
		return false
	}
	rightValue, err := decodeBridgeSetupComparableJSON(right)
	if err != nil {
		return false
	}
	return reflect.DeepEqual(leftValue, rightValue)
}

func decodeBridgeSetupComparableJSON(raw json.RawMessage) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func firstBridgeSetupValue(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
