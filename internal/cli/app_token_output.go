package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
)

func renderAppTokenExecutionError(err error) (string, bool) {
	apiErr, ok := errors.AsType[*daemonAPIError](err)
	if !ok || !strings.HasPrefix(apiErr.payload.Code, "app_token_") {
		return "", false
	}
	return fmt.Sprintf("error: %s (%s)", apiErr.Error(), apiErr.payload.Code), true
}

func appTokenIssuedOutput(result contract.AppTokenIssuedResponse) outputBundle {
	return simpleGatewayOutput(result, func() string {
		return fmt.Sprintf(
			"Created app token %q (%s)\nOrigins: %s\n\nToken (shown once, store it now):\n%s\n\n"+
				"Anyone with this token can use the allowed routes, including prompting agents that act on this\n"+
				"machine. A token in browser code is readable by anyone who loads that page.\n"+
				"See https://compozy.com/docs/apps/browser#security",
			result.AppToken.Name, result.AppToken.ID, appTokenOrigins(result.AppToken), result.Token,
		)
	}, func() string {
		return renderToonObject("app_token", append(appTokenFields(), "token"),
			append(appTokenValues(result.AppToken), result.Token))
	})
}

func appTokenUpdatedOutput(result contract.AppTokenResponse) outputBundle {
	return simpleGatewayOutput(result, func() string {
		return fmt.Sprintf("Updated app token %q (%s)\nOrigins: %s",
			result.AppToken.Name, result.AppToken.ID, appTokenOrigins(result.AppToken))
	}, func() string {
		return renderToonObject("app_token", appTokenFields(), appTokenValues(result.AppToken))
	})
}

func appTokenRevokedOutput(result contract.AppTokenRevokeResponse) outputBundle {
	return simpleGatewayOutput(result, func() string {
		stream := "streams"
		if result.Canceled == 1 {
			stream = "stream"
		}
		return fmt.Sprintf("Revoked app token %q (%s). Closed %d open %s.",
			result.AppToken.Name, result.AppToken.ID, result.Canceled, stream)
	}, func() string {
		return renderToonObject("app_token", append(appTokenFields(), "changed", "canceled"),
			append(appTokenValues(result.AppToken), strconv.FormatBool(result.Changed), strconv.Itoa(result.Canceled)))
	})
}

func appTokensOutput(result contract.AppTokensResponse, now func() time.Time) outputBundle {
	return listBundle(result, result.AppTokens, "App tokens",
		[]string{"ID", windowManagerNameHeader, "ORIGINS", "LAST USED", cliStateHeader},
		"app_tokens", appTokenFields(),
		func(token contract.AppTokenPayload) []string {
			origins := "—"
			if len(token.Origins) > 0 {
				origins = strings.Join(token.Origins, ", ")
			}
			lastUsed := "never"
			if token.LastUsedAt != nil {
				lastUsed = formatAge(now, *token.LastUsedAt) + " ago"
			}
			state := "active"
			if token.RevokedAt != nil {
				state = gatewayRevokedState
			}
			return []string{token.ID, token.Name, origins, lastUsed, state}
		}, appTokenValues)
}

func appTokenOrigins(token contract.AppTokenPayload) string {
	if len(token.Origins) == 0 {
		return "none (non-browser use only)"
	}
	return strings.Join(token.Origins, ", ")
}

func appTokenFields() []string {
	return []string{"id", automationNameKey, "origins", "created_at", "last_used_at", "revoked_at"}
}

func appTokenValues(token contract.AppTokenPayload) []string {
	return []string{token.ID, token.Name, strings.Join(token.Origins, ", "), formatTime(token.CreatedAt),
		formatOptionalTime(token.LastUsedAt), formatOptionalTime(token.RevokedAt)}
}
