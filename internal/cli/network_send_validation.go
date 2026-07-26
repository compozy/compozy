package cli

import (
	"errors"
	"fmt"
	"strings"

	networkpkg "github.com/compozy/agh/internal/network"
)

const (
	networkDirectIDPattern = networkpkg.DirectIDPattern
	networkThreadIDPattern = networkpkg.ThreadIDPattern
)

func validateNetworkSendFlags(flags networkSendFlags) error {
	kind := strings.TrimSpace(flags.kind)
	if kind == networkSurfaceDirect {
		return errors.New("cli: --kind direct is not supported; use --surface direct with --kind say")
	}

	surface := strings.TrimSpace(flags.surface)
	threadID := strings.TrimSpace(flags.threadID)
	directID := strings.TrimSpace(flags.directID)
	workID := strings.TrimSpace(flags.workID)
	target := strings.TrimSpace(flags.to)
	if err := validateNetworkSendConversationFlags(kind, surface, threadID, directID); err != nil {
		return err
	}
	return validateNetworkSendKindScope(kind, surface, threadID, directID, workID, target)
}

func validateNetworkSendConversationFlags(kind string, surface string, threadID string, directID string) error {
	if threadID != "" && directID != "" {
		return errors.New("cli: --thread and --direct cannot be used together")
	}
	if surface == "" {
		if threadID != "" || directID != "" {
			return errors.New("cli: --surface is required when --thread or --direct is set")
		}
		if networkKindRequiresConversation(kind) {
			return fmt.Errorf("cli: --surface is required for --kind %s", kind)
		}
		return nil
	}

	switch surface {
	case networkSurfaceThread:
		if threadID == "" {
			return errors.New("cli: --thread is required when --surface thread is set")
		}
		if err := networkpkg.ValidateConversationID(threadID, networkThreadIDKey); err != nil {
			return fmt.Errorf("cli: --thread must match %s: %w", networkThreadIDPattern, err)
		}
		if directID != "" {
			return errors.New("cli: --direct cannot be used when --surface thread is set")
		}
	case networkSurfaceDirect:
		if directID == "" {
			return errors.New("cli: --direct is required when --surface direct is set")
		}
		if err := networkpkg.ValidateConversationID(directID, networkDirectIDKey); err != nil {
			return fmt.Errorf("cli: --direct must match %s: %w", networkDirectIDPattern, err)
		}
		if threadID != "" {
			return errors.New("cli: --thread cannot be used when --surface direct is set")
		}
	default:
		return errors.New("cli: --surface must be thread or direct")
	}
	return nil
}

func validateNetworkSendKindScope(
	kind string,
	surface string,
	threadID string,
	directID string,
	workID string,
	target string,
) error {
	if networkKindForbidsConversation(kind) && (surface != "" || threadID != "" || directID != "" || workID != "") {
		return fmt.Errorf("cli: --kind %s cannot include --surface, --thread, --direct, or --work", kind)
	}
	if networkKindRequiresWork(kind) && workID == "" {
		return fmt.Errorf("cli: --kind %s requires --work", kind)
	}
	if kind == networkKindCapability && target == "" {
		return errors.New("cli: --kind capability requires --to")
	}
	if kind == networkKindSay && workID != "" && target == "" {
		return errors.New("cli: --kind say with --work requires --to")
	}
	return nil
}

func networkKindForbidsConversation(kind string) bool {
	switch kind {
	case networkKindGreet, networkKindWhois:
		return true
	default:
		return false
	}
}

func networkKindRequiresWork(kind string) bool {
	switch kind {
	case networkKindCapability, networkKindReceipt, networkKindTrace:
		return true
	default:
		return false
	}
}

func networkKindRequiresConversation(kind string) bool {
	switch kind {
	case networkKindSay, networkKindCapability, networkKindReceipt, networkKindTrace:
		return true
	default:
		return false
	}
}
