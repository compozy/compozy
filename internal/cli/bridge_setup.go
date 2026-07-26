package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

const (
	bridgeSetupInstanceFlag               = "instance"
	bridgeSetupPrintOnlyFlag              = "print-only"
	bridgeSetupRevealGeneratedSecretsFlag = "reveal-generated-secrets"
	bridgeSetupSecretKindToken            = "token"
)

// BridgeWebhookRegistrationRecord is the provider-owned Telegram webhook registration result.
type BridgeWebhookRegistrationRecord = bridgepkg.BridgeWebhookRegistrationResponse

type bridgeSetupClient interface {
	CreateBridge(context.Context, CreateBridgeRequest) (BridgeRecord, error)
	GetBridge(context.Context, string) (BridgeRecord, error)
	ListBridgeSecretBindings(context.Context, string) ([]BridgeSecretBindingRecord, error)
	PutBridgeSecretBinding(
		context.Context,
		string,
		string,
		BridgeSecretBindingRequest,
	) (BridgeSecretBindingRecord, error)
	RegisterBridgeWebhook(context.Context, string) (BridgeWebhookRegistrationRecord, error)
}

type bridgeSetupResult struct {
	BridgeInstanceID    string                           `json:"bridge_instance_id"`
	Platform            string                           `json:"platform"`
	Status              bridgepkg.BridgeStatus           `json:"status"`
	SecretSlots         map[string]string                `json:"secret_slots"`
	Instructions        []string                         `json:"instructions"`
	VerificationCommand string                           `json:"verification_command,omitempty"`
	WebhookCommand      string                           `json:"webhook_command,omitempty"`
	InviteURL           string                           `json:"invite_url,omitempty"`
	WebhookRegistration *BridgeWebhookRegistrationRecord `json:"webhook_registration,omitempty"`
	GeneratedSecrets    map[string]string                `json:"generated_secrets,omitempty"`
	NextCommand         string                           `json:"next_command"`
}

type bridgeSetupExistingState struct {
	Bridge   *BridgeRecord
	Bindings map[string]BridgeSecretBindingRecord
}

type bridgeSetupSecretWrite struct {
	BindingName string
	Value       string
}

type bridgeSetupPlan struct {
	Descriptor       bridgeSetupDescriptor
	Existing         bridgeSetupExistingState
	CreateRequest    CreateBridgeRequest
	ProviderDetails  bridgeSetupProviderDetails
	SecretWrites     []bridgeSetupSecretWrite
	SecretSlots      map[string]string
	GeneratedSecrets map[string]string
}

func newBridgeSetupCommand(deps commandDeps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Configure a bridge instance and its write-only secrets",
	}
	for _, platform := range []string{
		bridgeSetupPlatformWhatsApp,
		bridgeSetupPlatformTelegram,
		bridgeSetupPlatformDiscord,
	} {
		cmd.AddCommand(newBridgeSetupPlatformCommand(deps, platform))
	}
	return cmd
}

func newBridgeSetupPlatformCommand(deps commandDeps, platform string) *cobra.Command {
	var instanceID string
	var printOnly bool
	var revealGeneratedSecrets bool
	cmd := &cobra.Command{
		Use:   platform,
		Short: "Configure a " + platform + " bridge",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runBridgeSetupCommand(
				cmd,
				deps,
				platform,
				instanceID,
				printOnly,
				revealGeneratedSecrets,
			)
		},
	}
	cmd.Flags().StringVar(
		&instanceID,
		bridgeSetupInstanceFlag,
		"",
		"Re-run setup for an existing bridge instance",
	)
	if platform == bridgeSetupPlatformTelegram {
		cmd.Flags().BoolVar(
			&printOnly,
			bridgeSetupPrintOnlyFlag,
			false,
			"Print a safe setWebhook curl template instead of registering through the daemon",
		)
	}
	cmd.Flags().BoolVar(
		&revealGeneratedSecrets,
		bridgeSetupRevealGeneratedSecretsFlag,
		false,
		"Reveal only secrets generated during this setup in JSON output",
	)
	return cmd
}

func runBridgeSetupCommand(
	cmd *cobra.Command,
	deps commandDeps,
	platform string,
	instanceFlag string,
	printOnly bool,
	revealGeneratedSecrets bool,
) error {
	descriptor, err := bridgeSetupDescriptorFor(platform)
	if err != nil {
		return err
	}
	daemonClient, err := clientFromDeps(deps)
	if err != nil {
		return err
	}
	client, ok := daemonClient.(bridgeSetupClient)
	if !ok {
		return errors.New("cli: daemon client does not support bridge setup operations")
	}

	input, existing, err := resolveBridgeSetupInput(cmd, deps, client, descriptor, instanceFlag)
	if err != nil {
		return err
	}

	plan, err := buildBridgeSetupPlan(descriptor, input, existing, deps.generateBridgeSetupSecret)
	if err != nil {
		return err
	}
	if err := validateBridgeSetupSecretDisclosure(cmd, revealGeneratedSecrets, plan.GeneratedSecrets); err != nil {
		return err
	}
	if err := validateBridgeSetupGeneratedSecretUse(
		platform,
		printOnly,
		revealGeneratedSecrets,
		plan.GeneratedSecrets,
	); err != nil {
		return err
	}
	bridge, err := applyBridgeSetupPlan(cmd.Context(), client, plan)
	if err != nil {
		return err
	}
	result, err := finalizeBridgeSetup(
		cmd.Context(),
		client,
		bridge,
		plan,
		printOnly,
		revealGeneratedSecrets,
	)
	if err != nil {
		return err
	}
	return writeCommandOutput(cmd, bridgeSetupBundle(result))
}

func loadBridgeSetupExistingState(
	ctx context.Context,
	client bridgeSetupClient,
	platform string,
	instanceID string,
) (bridgeSetupExistingState, error) {
	state := bridgeSetupExistingState{Bindings: make(map[string]BridgeSecretBindingRecord)}
	id := strings.TrimSpace(instanceID)
	if id == "" {
		return state, nil
	}
	bridge, err := client.GetBridge(ctx, id)
	if err != nil {
		return bridgeSetupExistingState{}, fmt.Errorf("cli: get bridge %q for setup: %w", id, err)
	}
	if normalized := strings.ToLower(strings.TrimSpace(bridge.Platform)); normalized != platform {
		return bridgeSetupExistingState{}, fmt.Errorf(
			"cli: bridge %q uses platform %q, not %q",
			id,
			bridge.Platform,
			platform,
		)
	}
	bindings, err := client.ListBridgeSecretBindings(ctx, id)
	if err != nil {
		return bridgeSetupExistingState{}, fmt.Errorf("cli: list bridge %q secret bindings: %w", id, err)
	}
	state.Bridge = &bridge
	for _, binding := range bindings {
		state.Bindings[strings.TrimSpace(binding.BindingName)] = binding
	}
	return state, nil
}

func buildBridgeSetupPlan(
	descriptor bridgeSetupDescriptor,
	input bridgeSetupJSONInput,
	existing bridgeSetupExistingState,
	generateSecret bridgeSetupSecretGenerator,
) (bridgeSetupPlan, error) {
	if generateSecret == nil {
		return bridgeSetupPlan{}, errors.New("cli: bridge setup secret generator is required")
	}
	if err := normalizeBridgeSetupCommonInput(descriptor, &input, existing.Bridge); err != nil {
		return bridgeSetupPlan{}, err
	}
	providerDetails, err := buildBridgeSetupProviderDetails(descriptor.Platform, input, existing.Bridge)
	if err != nil {
		return bridgeSetupPlan{}, err
	}
	secretValues := bridgeSetupSecretValues(input)
	writes := make([]bridgeSetupSecretWrite, 0, len(descriptor.Secrets))
	slots := make(map[string]string, len(descriptor.Secrets))
	generatedSecrets := make(map[string]string)
	for _, field := range descriptor.Secrets {
		value := strings.TrimSpace(secretValues[field.Key])
		_, alreadyBound := existing.Bindings[field.BindingName]
		if value == bridgeSetupMaskedSecret || (value == "" && alreadyBound) {
			slots[field.BindingName] = bridgeSetupMaskedSecret
			continue
		}
		if value == "" && field.Generate {
			value, err = generateSecret()
			if err != nil {
				return bridgeSetupPlan{}, fmt.Errorf("cli: generate %s: %w", field.BindingName, err)
			}
			generatedSecrets[field.BindingName] = value
		}
		if value == "" && field.Required {
			return bridgeSetupPlan{}, fmt.Errorf("cli: %s is required", field.BindingName)
		}
		if field.Validate != nil && value != "" {
			if err := field.Validate(value); err != nil {
				return bridgeSetupPlan{}, fmt.Errorf("cli: invalid %s: %w", field.BindingName, err)
			}
		}
		if value != "" {
			writes = append(writes, bridgeSetupSecretWrite{BindingName: field.BindingName, Value: value})
			slots[field.BindingName] = bridgeSetupMaskedSecret
		}
	}
	routingPolicy, err := bridgeSetupRoutingPolicy(descriptor.Platform, input.RoutingPolicy, existing.Bridge)
	if err != nil {
		return bridgeSetupPlan{}, err
	}

	request := CreateBridgeRequest{
		Scope:          bridgepkg.Scope(input.Scope),
		WorkspaceID:    strings.TrimSpace(input.WorkspaceID),
		Platform:       descriptor.Platform,
		ExtensionName:  strings.TrimSpace(input.ExtensionName),
		DisplayName:    strings.TrimSpace(input.DisplayName),
		Enabled:        false,
		DMPolicy:       bridgepkg.BridgeDMPolicyOpen,
		RoutingPolicy:  routingPolicy,
		ProviderConfig: contract.BridgeProviderConfigPayload(providerDetails.ProviderConfig),
	}
	if existing.Bridge == nil {
		if err := validateBridgeCreatePayload(request); err != nil {
			return bridgeSetupPlan{}, err
		}
	}
	return bridgeSetupPlan{
		Descriptor:       descriptor,
		Existing:         existing,
		CreateRequest:    request,
		ProviderDetails:  providerDetails,
		SecretWrites:     writes,
		SecretSlots:      slots,
		GeneratedSecrets: generatedSecrets,
	}, nil
}

func normalizeBridgeSetupCommonInput(
	descriptor bridgeSetupDescriptor,
	input *bridgeSetupJSONInput,
	existing *BridgeRecord,
) error {
	if existing != nil {
		if input.Scope == "" {
			input.Scope = string(existing.Scope)
		}
		if input.WorkspaceID == "" {
			input.WorkspaceID = existing.WorkspaceID
		}
		if input.DisplayName == "" {
			input.DisplayName = existing.DisplayName
		}
		if input.ExtensionName == "" {
			input.ExtensionName = existing.ExtensionName
		}
	}
	if strings.TrimSpace(input.Scope) == "" {
		input.Scope = string(bridgepkg.ScopeGlobal)
	}
	if strings.TrimSpace(input.DisplayName) == "" {
		input.DisplayName = descriptor.DisplayName
	}
	if strings.TrimSpace(input.ExtensionName) == "" {
		input.ExtensionName = descriptor.ExtensionName
	}
	scope, err := parseBridgeScope(input.Scope)
	if err != nil {
		return err
	}
	input.Scope = string(scope)
	if err := bridgepkg.ValidateScopeWorkspaceID(scope, input.WorkspaceID); err != nil {
		return fmt.Errorf("cli: invalid bridge setup ownership: %w", err)
	}
	if existing != nil && (scope != existing.Scope.Normalize() ||
		strings.TrimSpace(input.WorkspaceID) != strings.TrimSpace(existing.WorkspaceID) ||
		strings.TrimSpace(input.ExtensionName) != strings.TrimSpace(existing.ExtensionName)) {
		return fmt.Errorf("cli: bridge %q ownership or extension differs from setup input", existing.ID)
	}
	return nil
}

func applyBridgeSetupPlan(
	ctx context.Context,
	client bridgeSetupClient,
	plan bridgeSetupPlan,
) (BridgeRecord, error) {
	var bridge BridgeRecord
	if plan.Existing.Bridge != nil {
		bridge = *plan.Existing.Bridge
	} else {
		created, err := client.CreateBridge(ctx, plan.CreateRequest)
		if err != nil {
			return BridgeRecord{}, fmt.Errorf("cli: create disabled %s bridge: %w", plan.Descriptor.Platform, err)
		}
		bridge = created
	}
	for _, secret := range plan.SecretWrites {
		binding := plan.Existing.Bindings[secret.BindingName]
		secretRef := strings.TrimSpace(binding.SecretRef)
		if secretRef == "" {
			secretRef = fmt.Sprintf("vault:bridges/%s/%s", bridge.ID, secret.BindingName)
		}
		kind := strings.TrimSpace(binding.Kind)
		if kind == "" {
			kind = bridgeSetupSecretKindToken
		}
		value := secret.Value
		if _, err := client.PutBridgeSecretBinding(ctx, bridge.ID, secret.BindingName, BridgeSecretBindingRequest{
			SecretRef:   secretRef,
			Kind:        kind,
			SecretValue: &value,
		}); err != nil {
			return BridgeRecord{}, fmt.Errorf(
				"cli: bind %s on bridge %q; completed bindings remain stored: %w",
				secret.BindingName,
				bridge.ID,
				err,
			)
		}
	}
	return bridge, nil
}

func finalizeBridgeSetup(
	ctx context.Context,
	client bridgeSetupClient,
	bridge BridgeRecord,
	plan bridgeSetupPlan,
	printOnly bool,
	revealGeneratedSecrets bool,
) (bridgeSetupResult, error) {
	result := bridgeSetupResult{
		BridgeInstanceID: bridge.ID,
		Platform:         plan.Descriptor.Platform,
		Status:           bridge.Status,
		SecretSlots:      plan.SecretSlots,
		InviteURL:        plan.ProviderDetails.InviteURL,
		NextCommand:      "agh bridge verify " + bridge.ID,
	}
	if revealGeneratedSecrets {
		result.GeneratedSecrets = cloneBridgeSetupSecrets(plan.GeneratedSecrets)
	}
	switch plan.Descriptor.Platform {
	case bridgeSetupPlatformWhatsApp:
		result.Instructions = []string{
			"In Meta App Dashboard, set the callback URL to " + plan.ProviderDetails.PublicURL + ".",
			"Use the bound verify_token and subscribe the webhook to messages.",
		}
		result.VerificationCommand = whatsappVerificationCommand(plan.ProviderDetails.PublicURL)
		if generated := result.GeneratedSecrets[bridgeSetupVerifyTokenKey]; generated != "" {
			result.VerificationCommand = whatsappVerificationCommandWithGeneratedToken(
				plan.ProviderDetails.PublicURL,
				generated,
			)
		}
	case bridgeSetupPlatformTelegram:
		if printOnly {
			result.WebhookCommand = telegramWebhookCommand(plan.ProviderDetails.PublicURL)
			result.Instructions = []string{
				"Export TELEGRAM_BOT_TOKEN and TELEGRAM_WEBHOOK_SECRET, then run webhook_command.",
			}
		} else {
			registration, err := client.RegisterBridgeWebhook(ctx, bridge.ID)
			if err != nil {
				return bridgeSetupResult{}, fmt.Errorf(
					"cli: register Telegram webhook for bridge %q: %w",
					bridge.ID,
					err,
				)
			}
			if err := registration.Validate(); err != nil {
				return bridgeSetupResult{}, fmt.Errorf("cli: invalid Telegram webhook registration result: %w", err)
			}
			result.WebhookRegistration = &registration
		}
	case bridgeSetupPlatformDiscord:
		result.Instructions = []string{
			"Open invite_url to add the bot with the configured scopes and permissions.",
			"Set the Discord Interactions Endpoint URL to " + plan.ProviderDetails.PublicURL + ".",
		}
	}
	return result, nil
}
