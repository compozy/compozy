package vault

import (
	"fmt"
	"strings"
)

// ValidateExtensionSecretRefOwner checks the extension segment of a binding reference.
// The input binder additionally enforces the exact workspace and profile instance.
func ValidateExtensionSecretRefOwner(ref, extension string) error {
	ref = NormalizeRef(ref)
	if err := ValidateSecretRefNamespace(ref, "extensions"); err != nil {
		return err
	}
	if strings.TrimSpace(extension) == "" {
		return fmt.Errorf("extension secret owner is required")
	}
	parts := strings.Split(strings.TrimPrefix(ref, "vault:extensions/"), "/")
	owner := ""
	switch {
	case len(parts) >= 3 && parts[0] == "global":
		owner = parts[1]
	case len(parts) >= 4 && parts[0] == "ws" && parts[1] != "":
		owner = parts[2]
	}
	if owner != collisionSafeVaultSegment(strings.TrimSpace(extension)) {
		return fmt.Errorf("secret reference is outside the extension owner")
	}
	return nil
}

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
