package cli

import "strings"

func bridgeSetupBundle(result bridgeSetupResult) outputBundle {
	return outputBundle{
		jsonValue: result,
		human: func() (string, error) {
			lines := []string{
				"Bridge setup",
				"Bridge: " + result.BridgeInstanceID,
				"Platform: " + result.Platform,
			}
			for _, slot := range sortedBridgeSetupSecretSlots(result.SecretSlots) {
				lines = append(lines, slot+": "+result.SecretSlots[slot])
			}
			lines = append(lines, result.Instructions...)
			for _, value := range []string{result.InviteURL, result.VerificationCommand, result.WebhookCommand} {
				if value != "" {
					lines = append(lines, value)
				}
			}
			lines = append(lines, result.NextCommand)
			return strings.Join(lines, "\n"), nil
		},
		toon: func() (string, error) {
			return renderToonObject(
				"bridge_setup",
				[]string{"bridge_instance_id", bridgePlatformKey, bridgeStatusKey, "next_command"},
				[]string{result.BridgeInstanceID, result.Platform, string(result.Status), result.NextCommand},
			), nil
		},
	}
}

func whatsappVerificationCommand(publicURL string) string {
	return "AGH_BRIDGE_WEBHOOK_URL=" + shellSingleQuote(publicURL) +
		" curl --get \"${AGH_BRIDGE_WEBHOOK_URL}\" " +
		"--data-urlencode \"hub.mode=subscribe\" " +
		"--data-urlencode \"hub.verify_token=${WHATSAPP_VERIFY_TOKEN}\" " +
		"--data-urlencode \"hub.challenge=hello\""
}

func whatsappVerificationCommandWithGeneratedToken(publicURL string, verifyToken string) string {
	return "WHATSAPP_VERIFY_TOKEN=" + shellSingleQuote(verifyToken) + " " +
		whatsappVerificationCommand(publicURL)
}

func telegramWebhookCommand(publicURL string) string {
	return "AGH_BRIDGE_WEBHOOK_URL=" + shellSingleQuote(publicURL) +
		" curl --request POST " +
		"\"https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/setWebhook\" " +
		"--data-urlencode \"url=${AGH_BRIDGE_WEBHOOK_URL}\" " +
		"--data-urlencode \"secret_token=${TELEGRAM_WEBHOOK_SECRET}\" " +
		"--data \"drop_pending_updates=true\""
}

func shellSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}
