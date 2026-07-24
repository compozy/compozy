package taskgroups

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"strings"
)

// SelectionFingerprint returns the stable identity of one initiative task-group selection.
func SelectionFingerprint(initiative string, taskGroupIDs []string, planChecksum string) string {
	sortedIDs := slices.Clone(taskGroupIDs)
	slices.Sort(sortedIDs)
	sum := sha256.Sum256([]byte(
		initiative + "\n" + strings.Join(sortedIDs, ",") + "\n" + planChecksum,
	))
	return hex.EncodeToString(sum[:])
}
