package globaldb

import (
	"encoding/json"
	"fmt"
	"maps"

	taskpkg "github.com/compozy/compozy/internal/task"
)

func loopTaskRecoveryMetadata(
	source json.RawMessage,
	override json.RawMessage,
	current loopNodeRunMetadata,
) (json.RawMessage, error) {
	metadata := make(map[string]any)
	if err := mergeLoopTaskMetadata(metadata, source); err != nil {
		return nil, err
	}
	if err := mergeLoopTaskMetadata(metadata, override); err != nil {
		return nil, err
	}
	metadata["generation"] = current.Generation
	metadata["node_id"] = current.NodeID
	metadata["item_index"] = current.ItemIndex
	metadata["attempt"] = current.Attempt + 1
	metadata["epoch"] = current.Epoch + 1
	for _, key := range []string{
		loopRunEventPayloadKeyFailure,
		"continuation_kind",
		"resume_from_task_run_id",
		"resume_from_session_id",
		"death_resume_checkpoint",
	} {
		delete(metadata, key)
	}
	encoded, err := json.Marshal(metadata)
	if err != nil {
		return nil, fmt.Errorf("store: marshal Loop task recovery metadata: %w", err)
	}
	if err := taskpkg.ValidateMetadataSize(encoded, "recover_run.metadata"); err != nil {
		return nil, err
	}
	return encoded, nil
}

func mergeLoopTaskMetadata(target map[string]any, raw json.RawMessage) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var values map[string]any
	if err := json.Unmarshal(raw, &values); err != nil {
		return fmt.Errorf("store: decode Loop task recovery metadata: %w", err)
	}
	if values == nil {
		return fmt.Errorf("%w: Loop task recovery metadata must be an object", taskpkg.ErrValidation)
	}
	maps.Copy(target, values)
	return nil
}
