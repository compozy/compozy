package vault

import "strings"

// ExtensionSecretOwnerPrefix returns the instance-qualified extension Vault prefix.
func ExtensionSecretOwnerPrefix(extension, workspaceID string) string {
	extensionSegment := collisionSafeMCPSegment(strings.TrimSpace(extension))
	workspace := strings.TrimSpace(workspaceID)
	if workspace == "" {
		return "vault:extensions/global/" + extensionSegment + "/"
	}
	return "vault:extensions/ws/" + collisionSafeMCPSegment(workspace) + "/" + extensionSegment + "/"
}

// ExtensionSecretRef returns the conventional Vault ref for one extension env binding.
func ExtensionSecretRef(extension, workspaceID, envName string) string {
	return ExtensionSecretOwnerPrefix(extension, workspaceID) +
		"env/" + collisionSafeMCPSegment(strings.TrimSpace(envName))
}
