package protocol

import (
	"slices"
	"strings"
)

const (
	// CapabilityProvideMemoryBackend is the provide surface for daemon-managed memory backends.
	CapabilityProvideMemoryBackend = "memory.backend"
	// CapabilityProvideBridgeAdapter is the provide surface for bridge-capable adapter extensions.
	CapabilityProvideBridgeAdapter = "bridge.adapter"
	// CapabilityToolProvider is the provide surface for executable extension-host tools.
	CapabilityToolProvider = "tool.provider"
	// CapabilityProvideModelSource is the provide surface for model catalog source rows.
	CapabilityProvideModelSource = "model.source"
	// CapabilityProvideWatchSource is the provide surface for loop watch-source polling.
	CapabilityProvideWatchSource = "loop.watch_source"
)

// ExtensionServiceMethod identifies one Compozy -> extension capability service request.
type ExtensionServiceMethod string

const (
	ExtensionServiceMethodMemoryStore    ExtensionServiceMethod = "memory/store"
	ExtensionServiceMethodMemoryRecall   ExtensionServiceMethod = "memory/recall"
	ExtensionServiceMethodMemoryForget   ExtensionServiceMethod = "memory/forget"
	ExtensionServiceMethodBridgesDeliver ExtensionServiceMethod = "bridges/deliver"
	ExtensionServiceMethodBridgeTargets  ExtensionServiceMethod = "bridges/targets/snapshot"
	ExtensionServiceMethodProvideTools   ExtensionServiceMethod = "provide_tools"
	ExtensionServiceMethodToolsCall      ExtensionServiceMethod = "tools/call"
	ExtensionServiceMethodModelsList     ExtensionServiceMethod = "models/list"
	ExtensionServiceMethodWatchPoll      ExtensionServiceMethod = "watch/poll"
)

var capabilityServiceMethods = map[string][]ExtensionServiceMethod{
	CapabilityProvideMemoryBackend: {
		ExtensionServiceMethodMemoryStore,
		ExtensionServiceMethodMemoryRecall,
		ExtensionServiceMethodMemoryForget,
	},
	CapabilityProvideBridgeAdapter: {
		ExtensionServiceMethodBridgesDeliver,
		ExtensionServiceMethodBridgeTargets,
	},
	CapabilityToolProvider: {
		ExtensionServiceMethodProvideTools,
		ExtensionServiceMethodToolsCall,
	},
	CapabilityProvideModelSource: {
		ExtensionServiceMethodModelsList,
	},
	CapabilityProvideWatchSource: {
		ExtensionServiceMethodWatchPoll,
	},
}

// ProvideCapabilities returns every daemon-recognized provide capability in stable order.
func ProvideCapabilities() []string {
	provides := make([]string, 0, len(capabilityServiceMethods))
	for provide := range capabilityServiceMethods {
		provides = append(provides, provide)
	}
	slices.Sort(provides)
	return provides
}

// RequiredServiceMethods returns the service methods required by one provide capability.
func RequiredServiceMethods(provide string) []string {
	methods := capabilityServiceMethods[strings.TrimSpace(provide)]
	if len(methods) == 0 {
		return nil
	}
	result := make([]string, 0, len(methods))
	for _, method := range methods {
		result = append(result, string(method))
	}
	slices.Sort(result)
	return result
}

// CapabilityServiceMethods returns the negotiated Compozy -> extension service methods
// enabled by the declared provide surfaces.
func CapabilityServiceMethods(provides []string) []string {
	if len(provides) == 0 {
		return nil
	}

	seen := make(map[string]struct{})
	methods := make([]string, 0)
	for _, provide := range normalizeUniqueStrings(provides) {
		for _, method := range RequiredServiceMethods(provide) {
			if _, ok := seen[method]; ok {
				continue
			}
			seen[method] = struct{}{}
			methods = append(methods, method)
		}
	}
	if len(methods) == 0 {
		return nil
	}
	slices.Sort(methods)
	return methods
}

func normalizeUniqueStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(values))
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		normalized = append(normalized, trimmed)
	}
	if len(normalized) == 0 {
		return nil
	}
	slices.Sort(normalized)
	return normalized
}
