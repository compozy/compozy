package vault

import "strings"

// ExtensionSecretOwnerPrefix returns the instance-qualified extension Vault prefix ending in a slash.
func ExtensionSecretOwnerPrefix(extension, workspaceID string) string {
	extensionSegment := collisionSafeVaultSegment(strings.TrimSpace(extension))
	workspace := strings.TrimSpace(workspaceID)
	if workspace == "" {
		return "vault:extensions/global/" + extensionSegment + "/"
	}
	return "vault:extensions/ws/" + collisionSafeVaultSegment(workspace) + "/" + extensionSegment + "/"
}

// ExtensionSecretRef returns the conventional Vault ref for one extension env binding.
func ExtensionSecretRef(extension, workspaceID, envName string) string {
	return ExtensionSecretOwnerPrefix(extension, workspaceID) +
		"env/" + collisionSafeVaultSegment(strings.TrimSpace(envName))
}

// ExtensionProfileSecretOwnerPrefix returns the profile-aware binding owner prefix.
// An empty profile keeps the shared user-layer namespace.
func ExtensionProfileSecretOwnerPrefix(extension, profileID, workspaceID string) string {
	profileID = strings.TrimSpace(profileID)
	if profileID == "" {
		return ExtensionSecretOwnerPrefix(extension, workspaceID)
	}
	return ExtensionSecretOwnerPrefix(extension, workspaceID) +
		"profiles/" + collisionSafeVaultSegment(profileID) + "/"
}

// ExtensionProfileSecretRef returns the conventional ref for one profile-aware binding.
func ExtensionProfileSecretRef(extension, profileID, workspaceID, envName string) string {
	return ExtensionProfileSecretOwnerPrefix(extension, profileID, workspaceID) +
		"env/" + collisionSafeVaultSegment(strings.TrimSpace(envName))
}
