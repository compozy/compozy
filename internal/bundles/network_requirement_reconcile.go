package bundles

import (
	"context"
	"strings"
)

func (s *Service) reconcileNetworkRequirementDigests(
	ctx context.Context,
	activations []Activation,
) (bool, error) {
	changed := false
	for i := range activations {
		digest, digestErr := s.networkRequirementDigestForExtension(ctx, activations[i].ExtensionName)
		if digestErr != nil {
			return changed, digestErr
		}
		updated := previewNetworkRequirement(activations[i], digest)
		if updated.NetworkRequirementDigest != activations[i].NetworkRequirementDigest ||
			updated.ConfirmedBy != activations[i].ConfirmedBy ||
			updated.ConfirmedAt != activations[i].ConfirmedAt {
			updated.UpdatedAt = s.now().UTC()
			if updateErr := s.store.UpdateBundleActivation(ctx, updated); updateErr != nil {
				return changed, updateErr
			}
			updated.Version = activations[i].Version + 1
			activations[i] = updated
			changed = true
			if strings.TrimSpace(digest) != "" {
				return changed, networkRequirementConfirmationError(digest)
			}
			continue
		}
		if strings.TrimSpace(digest) != "" &&
			(strings.TrimSpace(updated.ConfirmedBy) == "" || strings.TrimSpace(updated.ConfirmedAt) == "") {
			return changed, networkRequirementConfirmationError(digest)
		}
	}
	return changed, nil
}
