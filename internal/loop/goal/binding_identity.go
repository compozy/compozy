package goal

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

func deterministicBindingIdentity(key TurnKey, handle string, epoch int64) (string, string) {
	material := fmt.Sprintf(
		"%s\x00%s\x00%d\x00%s\x00%d\x00%s\x00%d",
		key.LoopRunID,
		key.WorkspaceID,
		key.Generation,
		key.NodeID,
		key.ItemIndex,
		handle,
		epoch,
	)
	digest := sha256.Sum256([]byte(material))
	hexDigest := hex.EncodeToString(digest[:])
	return "bind_" + hexDigest[:32], "sess_" + hexDigest[:32]
}

func activeReseedGrant(checkpoint Checkpoint) int64 {
	grant := checkpoint.ControlGrant
	if grant == nil || grant.Consumed || grant.Kind != ControlGrantReseed ||
		grant.Scope != ControlGrantScopeRotateBinding {
		return 0
	}
	return grant.ID
}
