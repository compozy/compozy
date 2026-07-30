package windowmanager

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func newPreviewIDGenerator(
	snapshot Snapshot,
	commandID CommandID,
	payload Command,
) (idGenerator, error) {
	seedInput := struct {
		WorkspaceID WorkspaceID `json:"workspace_id"`
		Revision    Revision    `json:"revision"`
		CommandID   CommandID   `json:"command_id"`
		Payload     Command     `json:"payload"`
	}{
		WorkspaceID: snapshot.WorkspaceID,
		Revision:    snapshot.Revision,
		CommandID:   commandID,
		Payload:     payload,
	}
	encoded, err := json.Marshal(seedInput)
	if err != nil {
		return nil, fmt.Errorf("encode preview ID seed: %w", err)
	}
	seed := sha256.Sum256(encoded)
	sequence := uint64(0)
	return func(kind string) (string, error) {
		sequence++
		material := fmt.Sprintf("%x:%s:%d", seed, kind, sequence)
		digest := sha256.Sum256([]byte(material))
		return kind + "-" + hex.EncodeToString(digest[:16]), nil
	}, nil
}
