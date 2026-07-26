package main

import (
	"strconv"
	"strings"

	bridgepkg "github.com/compozy/agh/internal/bridges/contract"
)

func allowDirectMessage(cfg *resolvedInstanceConfig, message telegramMessage) bool {
	if !isDirectChat(message.Chat.Type) {
		return true
	}
	if cfg == nil {
		return false
	}

	switch cfg.dmPolicy.Normalize() {
	case "", bridgepkg.BridgeDMPolicyOpen:
		return true
	case bridgepkg.BridgeDMPolicyAllowlist:
		return identityAllowed(cfg.allowUserIDs, cfg.allowUsernames, message.From)
	case bridgepkg.BridgeDMPolicyPairing:
		if identityAllowed(cfg.pairedUserIDs, cfg.pairedUsernames, message.From) {
			return true
		}
		return identityAllowed(cfg.allowUserIDs, cfg.allowUsernames, message.From)
	default:
		return false
	}
}

func identityAllowed(
	ids map[string]struct{},
	usernames map[string]struct{},
	user telegramUser,
) bool {
	if len(ids) == 0 && len(usernames) == 0 {
		return false
	}
	if _, ok := ids[strings.TrimSpace(strconv.FormatInt(user.ID, 10))]; ok {
		return true
	}
	if _, ok := usernames[normalizeUsername(user.Username)]; ok {
		return true
	}
	return false
}
