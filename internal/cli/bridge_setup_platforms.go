package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
)

const (
	bridgeSetupPlatformDiscord  = "discord"
	bridgeSetupPlatformTelegram = "telegram"
	bridgeSetupPlatformWhatsApp = "whatsapp"
)

const (
	bridgeSetupAccessTokenKey       = "access_token"
	bridgeSetupAppSecretKey         = "app_secret"
	bridgeSetupApplicationIDKey     = "application_id"
	bridgeSetupBotTokenKey          = "bot_token"
	bridgeSetupDisplayNameKey       = "display_name"
	bridgeSetupExtensionNameKey     = "extension_name"
	bridgeSetupInvitePermissionsKey = "invite_permissions"
	bridgeSetupInviteScopesKey      = "invite_scopes"
	bridgeSetupPhoneNumberIDKey     = "phone_number_id"
	bridgeSetupPublicKeyKey         = "public_key"
	bridgeSetupRoutingShapeKey      = "routing_shape"
	bridgeSetupScopeKey             = "scope"
	bridgeSetupVerifyTokenKey       = "verify_token"
	bridgeSetupWebhookConfigKey     = "webhook"
	bridgeSetupWebhookPathKey       = "webhook_path"
	bridgeSetupWebhookPublicURLKey  = "webhook_public_url" //nolint:gosec // Public URL contract key, not credential data.
	bridgeSetupWebhookSecretKey     = "webhook_secret"
	bridgeSetupWorkspaceIDKey       = "workspace_id"
)

const (
	bridgeSetupDiscordDefaultPermissions   = uint64(68672)
	bridgeSetupDiscordBotScope             = "bot"
	bridgeSetupMaskedSecret                = "********"
	bridgeSetupTelegramRoutingShapePrivate = "private"
	bridgeSetupTelegramRoutingShapeGroup   = "group"
	bridgeSetupTelegramRoutingShapeForum   = "forum"
)

var (
	bridgeSetupDigitsPattern        = regexp.MustCompile(`^\d+$`)
	bridgeSetupHexPattern           = regexp.MustCompile(`^[0-9a-fA-F]+$`)
	bridgeSetupTelegramTokenPattern = regexp.MustCompile(`^\d+:[A-Za-z0-9_-]{30,}$`)
)

type bridgeSetupSecretField struct {
	Key         string
	BindingName string
	Required    bool
	Generate    bool
	Validate    func(string) error
}

type bridgeSetupDescriptor struct {
	Platform      string
	DisplayName   string
	ExtensionName string
	Prompts       []bridgeSetupPrompt
	Secrets       []bridgeSetupSecretField
}

func bridgeSetupDescriptorFor(platform string) (bridgeSetupDescriptor, error) {
	normalized := strings.ToLower(strings.TrimSpace(platform))
	switch normalized {
	case bridgeSetupPlatformWhatsApp:
		return whatsAppBridgeSetupDescriptor(), nil
	case bridgeSetupPlatformTelegram:
		return telegramBridgeSetupDescriptor(), nil
	case bridgeSetupPlatformDiscord:
		return discordBridgeSetupDescriptor(), nil
	default:
		return bridgeSetupDescriptor{}, fmt.Errorf("cli: unsupported bridge setup platform %q", normalized)
	}
}

func whatsAppBridgeSetupDescriptor() bridgeSetupDescriptor {
	return bridgeSetupDescriptor{
		Platform:      bridgeSetupPlatformWhatsApp,
		DisplayName:   "WhatsApp bridge",
		ExtensionName: bridgeSetupPlatformWhatsApp,
		Prompts: append(bridgeSetupCommonPrompts(),
			bridgeSetupPrompt{
				Key: bridgeSetupPhoneNumberIDKey, Label: "Meta Phone Number ID",
				Required: true, Validate: validateWhatsAppPhoneNumberID,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupAccessTokenKey, Label: "Meta access token",
				Secret: true, Required: true, Validate: validateWhatsAppAccessToken,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupAppSecretKey, Label: "Meta app secret",
				Secret: true, Required: true, Validate: validateWhatsAppAppSecret,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupVerifyTokenKey, Label: "Meta webhook verify token",
				Secret: true, Required: true,
			},
		),
		Secrets: []bridgeSetupSecretField{
			{
				Key: bridgeSetupAccessTokenKey, BindingName: bridgeSetupAccessTokenKey,
				Required: true, Validate: validateWhatsAppAccessToken,
			},
			{
				Key: bridgeSetupAppSecretKey, BindingName: bridgeSetupAppSecretKey,
				Required: true, Validate: validateWhatsAppAppSecret,
			},
			{
				Key: bridgeSetupVerifyTokenKey, BindingName: bridgeSetupVerifyTokenKey,
				Required: true, Generate: true,
			},
		},
	}
}

func validateTelegramRoutingShape(value string) error {
	_, err := telegramRoutingPolicyForShape(value)
	return err
}

func telegramRoutingPolicyForShape(value string) (bridgepkg.RoutingPolicy, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case bridgeSetupTelegramRoutingShapePrivate:
		return bridgepkg.RoutingPolicy{IncludePeer: true}, nil
	case bridgeSetupTelegramRoutingShapeGroup:
		return bridgepkg.RoutingPolicy{IncludeGroup: true}, nil
	case bridgeSetupTelegramRoutingShapeForum:
		return bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true}, nil
	default:
		return bridgepkg.RoutingPolicy{}, errors.New(
			"telegram route shape must be private, group, or forum",
		)
	}
}

func telegramRoutingShapeForPolicy(policy bridgepkg.RoutingPolicy) (string, error) {
	switch policy {
	case bridgepkg.RoutingPolicy{IncludePeer: true}:
		return bridgeSetupTelegramRoutingShapePrivate, nil
	case bridgepkg.RoutingPolicy{IncludeGroup: true}:
		return bridgeSetupTelegramRoutingShapeGroup, nil
	case bridgepkg.RoutingPolicy{IncludeGroup: true, IncludeThread: true}:
		return bridgeSetupTelegramRoutingShapeForum, nil
	default:
		return "", errors.New(
			"cli: Telegram routing_policy must select exactly one supported shape: private, group, or forum",
		)
	}
}

func telegramBridgeSetupDescriptor() bridgeSetupDescriptor {
	return bridgeSetupDescriptor{
		Platform:      bridgeSetupPlatformTelegram,
		DisplayName:   "Telegram bridge",
		ExtensionName: bridgeSetupPlatformTelegram,
		Prompts: append(bridgeSetupCommonPrompts(),
			bridgeSetupPrompt{
				Key: bridgeSetupRoutingShapeKey, Label: "Telegram route shape (private, group, or forum)",
				Required: true, Validate: validateTelegramRoutingShape,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupBotTokenKey, Label: "BotFather bot token",
				Secret: true, Required: true, Validate: validateTelegramBotToken,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupWebhookSecretKey, Label: "Telegram webhook secret",
				Secret: true, Required: true,
			},
		),
		Secrets: []bridgeSetupSecretField{
			{
				Key: bridgeSetupBotTokenKey, BindingName: bridgeSetupBotTokenKey,
				Required: true, Validate: validateTelegramBotToken,
			},
			{
				Key: bridgeSetupWebhookSecretKey, BindingName: bridgeSetupWebhookSecretKey,
				Required: true, Generate: true,
			},
		},
	}
}

func discordBridgeSetupDescriptor() bridgeSetupDescriptor {
	botTokenValidator := validateRequiredSetupValue("Discord bot token")
	return bridgeSetupDescriptor{
		Platform:      bridgeSetupPlatformDiscord,
		DisplayName:   "Discord bridge",
		ExtensionName: bridgeSetupPlatformDiscord,
		Prompts: append(bridgeSetupCommonPrompts(),
			bridgeSetupPrompt{
				Key: bridgeSetupApplicationIDKey, Label: "Discord application ID",
				Required: true, Validate: validateDiscordApplicationID,
			},
			bridgeSetupPrompt{Key: bridgeSetupInviteScopesKey, Label: "Discord invite scopes", Required: true},
			bridgeSetupPrompt{
				Key: bridgeSetupInvitePermissionsKey, Label: "Discord permission bits",
				Required: true, Validate: validateDiscordPermissions,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupBotTokenKey, Label: "Discord bot token",
				Secret: true, Required: true, Validate: botTokenValidator,
			},
			bridgeSetupPrompt{
				Key: bridgeSetupPublicKeyKey, Label: "Discord Ed25519 public key",
				Secret: true, Required: true, Validate: validateDiscordPublicKey,
			},
		),
		Secrets: []bridgeSetupSecretField{
			{
				Key: bridgeSetupBotTokenKey, BindingName: bridgeSetupBotTokenKey,
				Required: true, Validate: botTokenValidator,
			},
			{
				Key: bridgeSetupPublicKeyKey, BindingName: bridgeSetupPublicKeyKey,
				Required: true, Validate: validateDiscordPublicKey,
			},
		},
	}
}

func bridgeSetupCommonPrompts() []bridgeSetupPrompt {
	return []bridgeSetupPrompt{
		{
			Key:      bridgeSetupScopeKey,
			Label:    "Bridge scope (global or workspace)",
			Required: true,
			Validate: func(value string) error {
				_, err := parseBridgeScope(value)
				return err
			},
		},
		{Key: bridgeSetupWorkspaceIDKey, Label: "Workspace ID"},
		{
			Key:      bridgeSetupDisplayNameKey,
			Label:    "Bridge display name",
			Required: true,
			Validate: validateRequiredSetupValue("bridge display name"),
		},
		{
			Key:      bridgeSetupExtensionNameKey,
			Label:    "Installed extension name",
			Required: true,
			Validate: validateRequiredSetupValue("extension name"),
		},
		{
			Key:      bridgeSetupWebhookPublicURLKey,
			Label:    "Public webhook URL",
			Required: true,
			Validate: validateBridgeSetupWebhookPublicURL,
		},
		{
			Key:      bridgeSetupWebhookPathKey,
			Label:    "Internal webhook path",
			Required: true,
			Validate: validateBridgeSetupWebhookPath,
		},
	}
}

func validateWhatsAppPhoneNumberID(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("phone number ID is required")
	}
	if !bridgeSetupDigitsPattern.MatchString(trimmed) {
		return errors.New("phone number ID must contain only digits")
	}
	if len(trimmed) >= 10 && len(trimmed) <= 12 {
		return errors.New(
			"that looks like a phone number; look below the From dropdown for Meta's 15-17 digit Phone Number ID",
		)
	}
	if len(trimmed) < 15 || len(trimmed) > 17 {
		return errors.New("phone number ID must contain 15-17 digits")
	}
	return nil
}

func validateWhatsAppAccessToken(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return errors.New("meta WhatsApp access token is required")
	}
	switch {
	case strings.HasPrefix(trimmed, "sk-"):
		return errors.New("that is an OpenAI key; Meta WhatsApp access tokens start with EAA")
	case strings.HasPrefix(trimmed, "xoxb-") || strings.HasPrefix(trimmed, "xoxp-"):
		return errors.New("that is a Slack token; Meta WhatsApp access tokens start with EAA")
	case strings.HasPrefix(trimmed, "ghp_") || strings.HasPrefix(trimmed, "gho_"):
		return errors.New("that is a GitHub token; Meta WhatsApp access tokens start with EAA")
	case !strings.HasPrefix(trimmed, "EAA"):
		return errors.New("meta WhatsApp access tokens must start with EAA")
	case len(trimmed) < 100:
		return fmt.Errorf(
			"meta WhatsApp access token is too short (%d characters; expected at least 100)",
			len(trimmed),
		)
	default:
		return nil
	}
}

func validateWhatsAppAppSecret(value string) error {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 32 {
		return fmt.Errorf(
			"meta app secret must contain exactly 32 hexadecimal characters (got %d)",
			len(trimmed),
		)
	}
	if !bridgeSetupHexPattern.MatchString(trimmed) {
		return errors.New("meta app secret must contain only hexadecimal characters")
	}
	return nil
}

func validateTelegramBotToken(value string) error {
	if !bridgeSetupTelegramTokenPattern.MatchString(strings.TrimSpace(value)) {
		return errors.New("telegram bot token must match <bot-id>:<30-or-more-token-characters>")
	}
	return nil
}

func validateDiscordApplicationID(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !bridgeSetupDigitsPattern.MatchString(trimmed) {
		return errors.New("discord application ID must contain only digits")
	}
	return nil
}

func validateDiscordPublicKey(value string) error {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) != 64 || !bridgeSetupHexPattern.MatchString(trimmed) {
		return errors.New("discord Ed25519 public key must contain exactly 64 hexadecimal characters")
	}
	return nil
}

func validateDiscordPermissions(value string) error {
	if _, err := strconv.ParseUint(strings.TrimSpace(value), 10, 64); err != nil {
		return fmt.Errorf("discord permission bits must be an unsigned decimal integer: %w", err)
	}
	return nil
}

func validateRequiredSetupValue(label string) func(string) error {
	return func(value string) error {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", strings.TrimSpace(label))
		}
		return nil
	}
}

func validateBridgeSetupWebhookPublicURL(value string) error {
	providerConfig, err := json.Marshal(map[string]any{
		bridgeSetupWebhookConfigKey: map[string]string{"public_url": strings.TrimSpace(value)},
	})
	if err != nil {
		return fmt.Errorf("encode webhook public URL for validation: %w", err)
	}
	instance := bridgepkg.BridgeInstance{ProviderConfig: providerConfig}
	if _, err := bridgepkg.WebhookPublicURL(instance); err != nil {
		return err
	}
	return nil
}

func validateBridgeSetupWebhookPath(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !strings.HasPrefix(trimmed, "/") {
		return errors.New("internal webhook path must start with /")
	}
	if parsed, err := url.ParseRequestURI(trimmed); err != nil || parsed.IsAbs() ||
		parsed.Host != "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("internal webhook path must be a valid absolute path")
	}
	return nil
}

func buildDiscordInviteURL(applicationID string, scopes []string, permissions uint64) (string, error) {
	if err := validateDiscordApplicationID(applicationID); err != nil {
		return "", err
	}
	normalizedScopes := normalizeDiscordInviteScopes(scopes)
	values := url.Values{}
	values.Set("client_id", strings.TrimSpace(applicationID))
	values.Set("permissions", strconv.FormatUint(permissions, 10))
	values.Set("scope", strings.Join(normalizedScopes, " "))
	return "https://discord.com/oauth2/authorize?" + values.Encode(), nil
}

func normalizeDiscordInviteScopes(scopes []string) []string {
	seen := make(map[string]struct{}, len(scopes)+2)
	normalized := make([]string, 0, len(scopes)+2)
	for _, scope := range scopes {
		for _, item := range strings.FieldsFunc(scope, func(r rune) bool { return r == ',' || r == ' ' }) {
			value := strings.TrimSpace(item)
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			normalized = append(normalized, value)
		}
	}
	for _, required := range []string{bridgeSetupDiscordBotScope, "applications.commands"} {
		if _, ok := seen[required]; !ok {
			normalized = append(normalized, required)
		}
	}
	return normalized
}

func sortedBridgeSetupSecretSlots(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
