package extensionpkg

import (
	"fmt"
	"strings"

	profilepkg "github.com/compozy/compozy/internal/profile"
	"github.com/compozy/compozy/internal/vault"
)

func validateManifestProfiles(manifest *Manifest) error {
	seen := make(map[string]struct{}, len(manifest.Profiles))
	for index, declaration := range manifest.Profiles {
		field := fmt.Sprintf("profiles[%d]", index)
		name, err := profilepkg.NormalizeName(declaration.Name)
		if err != nil {
			return &ManifestValidationError{Field: field + ".name", Value: declaration.Name, Message: err.Error()}
		}
		if _, duplicate := seen[name]; duplicate {
			return &ManifestValidationError{Field: field + ".name", Value: name, Message: "duplicate declared profile"}
		}
		seen[name] = struct{}{}
		if _, _, _, err := profilepkg.NormalizeIdentity(
			declaration.Color, declaration.Icon, declaration.Emoji,
		); err != nil {
			return &ManifestValidationError{Field: field, Message: err.Error()}
		}
		credentialSeen := make(map[string]struct{}, len(declaration.Credentials))
		for credentialIndex, credential := range declaration.Credentials {
			credentialField := fmt.Sprintf("%s.credentials[%d]", field, credentialIndex)
			provider := strings.TrimSpace(credential.Provider)
			slot := strings.TrimSpace(credential.Slot)
			ref := fmt.Sprintf(
				"vault:profiles/%s/providers/%s/%s",
				name,
				provider,
				slot,
			)
			if _, err := vault.ParseProfileSecretRef(ref); err != nil {
				return &ManifestValidationError{Field: credentialField, Message: err.Error()}
			}
			key := provider + "\x00" + slot
			if _, duplicate := credentialSeen[key]; duplicate {
				return &ManifestValidationError{Field: credentialField, Message: "duplicate credential requirement"}
			}
			credentialSeen[key] = struct{}{}
		}
	}
	return validateManifestPlacements(manifest)
}

func validateManifestPlacements(manifest *Manifest) error {
	for _, placement := range manifestPlacements(manifest) {
		name := strings.TrimSpace(placement.profile)
		if name == "" || name == hostAPIBridgesDefaultKey {
			continue
		}
		if _, err := profilepkg.NormalizeName(name); err != nil {
			return &ManifestValidationError{Field: placement.field, Value: name, Message: err.Error()}
		}
	}
	return nil
}

func manifestPlacements(manifest *Manifest) []manifestPlacementEntry {
	placements := make([]manifestPlacementEntry, 0)
	walkManifestPlacements(manifest, func(entry manifestPlacementEntry) {
		placements = append(placements, entry)
	})
	return placements
}
