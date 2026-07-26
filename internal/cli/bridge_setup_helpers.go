package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

func decodeBridgeSetupJSON(reader io.Reader) (bridgeSetupJSONInput, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var input bridgeSetupJSONInput
	if err := decoder.Decode(&input); err != nil {
		return bridgeSetupJSONInput{}, fmt.Errorf("cli: decode bridge setup JSON: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return bridgeSetupJSONInput{}, errors.New("cli: bridge setup JSON must contain exactly one object")
		}
		return bridgeSetupJSONInput{}, fmt.Errorf("cli: decode trailing bridge setup JSON: %w", err)
	}
	return input, nil
}

func resolveBridgeSetupInput(
	cmd *cobra.Command,
	deps commandDeps,
	client bridgeSetupClient,
	descriptor bridgeSetupDescriptor,
	instanceFlag string,
) (bridgeSetupJSONInput, bridgeSetupExistingState, error) {
	jsonMode, err := bridgeSetupJSONMode(cmd)
	if err != nil {
		return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, err
	}
	input := bridgeSetupJSONInput{InstanceID: strings.TrimSpace(instanceFlag)}
	if jsonMode {
		input, err = decodeBridgeSetupJSON(cmd.InOrStdin())
		if err != nil {
			return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, err
		}
		if flagID := strings.TrimSpace(instanceFlag); flagID != "" {
			if inputID := strings.TrimSpace(input.InstanceID); inputID != "" && inputID != flagID {
				return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, errors.New(
					"cli: --instance and JSON instance_id must identify the same bridge",
				)
			}
			input.InstanceID = flagID
		}
	}

	existing, err := loadBridgeSetupExistingState(
		cmd.Context(),
		client,
		descriptor.Platform,
		input.InstanceID,
	)
	if err != nil {
		return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, err
	}
	if jsonMode {
		return input, existing, nil
	}
	defaults, err := bridgeSetupWizardDefaults(
		descriptor,
		existing.Bridge,
		bridgeSetupBindings(existing),
	)
	if err != nil {
		return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, err
	}
	selection, err := deps.runBridgeSetupWizard(cmd.Context(), bridgeSetupWizardInput{
		Platform: descriptor.Platform,
		Prompts:  descriptor.Prompts,
		Defaults: defaults,
		Input:    cmd.InOrStdin(),
		Output:   cmd.ErrOrStderr(),
	})
	if err != nil {
		return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, err
	}
	input, err = bridgeSetupInputFromWizard(descriptor, selection, existing.Bridge)
	if err != nil {
		return bridgeSetupJSONInput{}, bridgeSetupExistingState{}, err
	}
	return input, existing, nil
}

func bridgeSetupJSONMode(cmd *cobra.Command) (bool, error) {
	value, err := cmd.Flags().GetBool(jsonFlagName)
	if err != nil {
		return false, fmt.Errorf("cli: read --json flag: %w", err)
	}
	return value, nil
}

func bridgeSetupBindings(state bridgeSetupExistingState) []BridgeSecretBindingRecord {
	bindings := make([]BridgeSecretBindingRecord, 0, len(state.Bindings))
	for _, binding := range state.Bindings {
		bindings = append(bindings, binding)
	}
	return bindings
}

func bridgeSetupRoutingPolicy(
	platform string,
	requested *bridgepkg.RoutingPolicy,
	existing *BridgeRecord,
) (bridgepkg.RoutingPolicy, error) {
	policy := bridgeSetupDefaultRoutingPolicy(platform)
	if existing != nil {
		policy = existing.RoutingPolicy
	}
	if requested != nil {
		policy = *requested
	}
	if err := policy.Validate(); err != nil {
		return bridgepkg.RoutingPolicy{}, fmt.Errorf("cli: invalid bridge setup routing policy: %w", err)
	}
	if platform == bridgeSetupPlatformTelegram {
		if _, err := telegramRoutingShapeForPolicy(policy); err != nil {
			return bridgepkg.RoutingPolicy{}, err
		}
	}
	return policy, nil
}

func bridgeSetupDefaultRoutingPolicy(platform string) bridgepkg.RoutingPolicy {
	switch platform {
	case bridgeSetupPlatformTelegram:
		return bridgepkg.RoutingPolicy{IncludeThread: true, IncludeGroup: true}
	case bridgeSetupPlatformDiscord:
		return bridgepkg.RoutingPolicy{IncludeGroup: true}
	case bridgeSetupPlatformWhatsApp:
		return bridgepkg.RoutingPolicy{IncludePeer: true}
	default:
		return bridgepkg.RoutingPolicy{}
	}
}

func validateBridgeSetupSecretDisclosure(
	cmd *cobra.Command,
	reveal bool,
	generated map[string]string,
) error {
	if !reveal {
		return nil
	}
	mode, err := resolveOutputFormat(cmd)
	if err != nil {
		return err
	}
	if mode != OutputJSON {
		return errors.New("cli: --reveal-generated-secrets requires JSON output")
	}
	if len(generated) == 0 {
		return errors.New("cli: --reveal-generated-secrets requires a secret generated during this setup")
	}
	return nil
}

func validateBridgeSetupGeneratedSecretUse(
	platform string,
	printOnly bool,
	reveal bool,
	generated map[string]string,
) error {
	if reveal {
		return nil
	}
	switch platform {
	case bridgeSetupPlatformWhatsApp:
		if generated[bridgeSetupVerifyTokenKey] != "" {
			return errors.New(
				"cli: generated verify_token would become write-only before Meta setup; " +
					"provide verify_token or rerun with --json --reveal-generated-secrets",
			)
		}
	case bridgeSetupPlatformTelegram:
		if printOnly && generated[bridgeSetupWebhookSecretKey] != "" {
			return errors.New(
				"cli: generated webhook_secret cannot be used with --print-only unless revealed; " +
					"provide webhook_secret or rerun with --json --reveal-generated-secrets",
			)
		}
	}
	return nil
}

func cloneBridgeSetupSecrets(values map[string]string) map[string]string {
	cloned := make(map[string]string, len(values))
	maps.Copy(cloned, values)
	return cloned
}
