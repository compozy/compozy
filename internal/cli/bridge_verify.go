package cli

import (
	"errors"
	"fmt"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges"
	"github.com/spf13/cobra"
)

var errBridgeVerificationFailed = errors.New("cli: one or more bridge verification checks failed")

func newBridgeVerifyCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:   "verify <id>",
		Short: "Run live provider checks for one bridge instance",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.VerifyBridge(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if err := writeCommandOutput(cmd, bridgeVerifyBundle(result)); err != nil {
				return err
			}
			return bridgeVerificationError(result.Checks)
		},
	}
}

func newBridgeSendTestCommand(deps commandDeps) *cobra.Command {
	var (
		message  string
		peerID   string
		threadID string
		groupID  string
		modeRaw  string
	)

	cmd := &cobra.Command{
		Use:   "send-test <id>",
		Short: "Send one real message through a bridge provider",
		Args:  exactOneNonBlankArg(),
		RunE: func(cmd *cobra.Command, args []string) error {
			trimmedMessage := strings.TrimSpace(message)
			if trimmedMessage == "" {
				return errors.New("cli: --message is required")
			}
			mode := bridgepkg.DeliveryMode(strings.TrimSpace(modeRaw)).Normalize()
			if mode != "" {
				if err := mode.Validate(); err != nil {
					return err
				}
			}

			client, err := clientFromDeps(deps)
			if err != nil {
				return err
			}
			result, err := client.SendBridgeTest(cmd.Context(), args[0], BridgeSendTestRequest{
				Message: trimmedMessage,
				Target: BridgeDeliveryTargetInput{
					PeerID:   strings.TrimSpace(peerID),
					ThreadID: strings.TrimSpace(threadID),
					GroupID:  strings.TrimSpace(groupID),
					Mode:     mode,
				},
			})
			if err != nil {
				return err
			}
			return writeCommandOutput(cmd, bridgeSendTestBundle(result))
		},
	}
	cmd.Flags().StringVar(&message, bridgeMessageKey, "", "Message to send through the provider")
	cmd.Flags().StringVar(&peerID, "peer-id", "", "Override target peer ID")
	cmd.Flags().StringVar(&threadID, "thread-id", "", "Override target thread ID")
	cmd.Flags().StringVar(&groupID, "group-id", "", "Override target group ID")
	cmd.Flags().StringVar(&modeRaw, bridgeModeKey, "", "Delivery mode: direct-send or reply")
	mustMarkFlagRequired(cmd, bridgeMessageKey)
	return cmd
}

func bridgeVerifyBundle(result BridgeVerifyRecord) outputBundle {
	return listBundle(
		result,
		result.Checks,
		"Bridge Verification",
		[]string{"CHECK", cliStatusHeader, "REMEDIATION"},
		"bridge_checks",
		[]string{"check", lifecycleStatusKey, "remediation"},
		func(check bridgepkg.BridgeCheckRecord) []string {
			return []string{
				check.Check,
				string(check.Status),
				stringOrDash(check.Remediation),
			}
		},
		func(check bridgepkg.BridgeCheckRecord) []string {
			return []string{check.Check, string(check.Status), check.Remediation}
		},
	)
}

func bridgeSendTestBundle(result BridgeSendTestRecord) outputBundle {
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			deliveryValues := []keyValue{
				{Label: automationStatusValue, Value: stringOrDash(string(result.Status))},
				{Label: bridgeBridgeValue, Value: stringOrDash(result.BridgeInstanceID)},
				{Label: "Delivery", Value: stringOrDash(result.DeliveryID)},
			}
			if strings.TrimSpace(result.RemoteMessageID) != "" {
				deliveryValues = append(deliveryValues, keyValue{
					Label: "Remote Message",
					Value: result.RemoteMessageID,
				})
			}
			if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
				deliveryValues = append(deliveryValues, keyValue{
					Label: "Error",
					Value: result.Error.Message,
				})
			}
			return renderHumanBlocks(
				renderHumanSection("Send Test", deliveryValues),
				renderHumanSection("Delivery Target", []keyValue{
					{Label: taskPeerValue, Value: stringOrDash(result.DeliveryTarget.PeerID)},
					{Label: taskThreadValue, Value: stringOrDash(result.DeliveryTarget.ThreadID)},
					{Label: taskGroupValue, Value: stringOrDash(result.DeliveryTarget.GroupID)},
					{Label: bridgeModeValue, Value: stringOrDash(string(result.DeliveryTarget.Mode))},
				}),
			), nil
		},
		toon: func() (string, error) {
			fields := []string{
				bridgeStatusKey,
				taskBridgeInstanceIDKey,
				"delivery_id",
				bridgePeerIDKey,
				bridgeThreadIDKey,
				bridgeGroupIDKey,
				bridgeModeKey,
			}
			values := []string{
				string(result.Status),
				result.BridgeInstanceID,
				result.DeliveryID,
				result.DeliveryTarget.PeerID,
				result.DeliveryTarget.ThreadID,
				result.DeliveryTarget.GroupID,
				string(result.DeliveryTarget.Mode),
			}
			if strings.TrimSpace(result.RemoteMessageID) != "" {
				fields = append(fields, "remote_message_id")
				values = append(values, result.RemoteMessageID)
			}
			if result.Error != nil && strings.TrimSpace(result.Error.Message) != "" {
				fields = append(fields, "error_message")
				values = append(values, result.Error.Message)
			}
			return renderToonObject("bridge_send_test", fields, values), nil
		},
	}
}

func bridgeVerificationError(checks []bridgepkg.BridgeCheckRecord) error {
	failed := 0
	for _, check := range checks {
		if check.Status == bridgepkg.BridgeCheckStatusFail {
			failed++
		}
	}
	if failed == 0 {
		return nil
	}
	return fmt.Errorf("%w: %d failed", errBridgeVerificationFailed, failed)
}
