package bundles

import (
	"context"
	"strings"
)

func activationSpecDrift(storedHash string, currentHash string) bool {
	return strings.TrimSpace(storedHash) != strings.TrimSpace(currentHash)
}

func (s *Service) warnSpecHashDrift(ctx context.Context, activation Activation, currentHash string) {
	storedHash := strings.TrimSpace(activation.SpecContentHash)
	currentHash = strings.TrimSpace(currentHash)
	switch {
	case storedHash == "":
		s.logger.WarnContext(
			ctx,
			"bundles.activation.spec_hash_missing",
			"activation_id", strings.TrimSpace(activation.ID),
			"extension_name", strings.TrimSpace(activation.ExtensionName),
			"bundle_name", strings.TrimSpace(activation.BundleName),
			"profile_name", strings.TrimSpace(activation.ProfileName),
			"current_hash", currentHash,
		)
	case storedHash != currentHash:
		s.logger.WarnContext(
			ctx,
			"bundles.activation.spec_hash_drift",
			"activation_id", strings.TrimSpace(activation.ID),
			"extension_name", strings.TrimSpace(activation.ExtensionName),
			"bundle_name", strings.TrimSpace(activation.BundleName),
			"profile_name", strings.TrimSpace(activation.ProfileName),
			"stored_hash", storedHash,
			"current_hash", currentHash,
		)
	}
}
