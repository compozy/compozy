package daemon

import (
	"bytes"

	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	taskpkg "github.com/compozy/agh/internal/task"
)

func validateDetachedHarnessTaskMatch(
	record taskpkg.Task,
	req normalizedDetachedHarnessSubmitRequest,
	actor taskpkg.ActorContext,
	expected detachedHarnessTaskMetadata,
) error {
	if record.Scope != req.Scope ||
		record.WorkspaceID != req.WorkspaceID ||
		record.Title != req.Summary ||
		record.Description != req.Description ||
		record.CreatedBy != actor.Actor ||
		record.Origin != actor.Origin {
		return fmt.Errorf(
			"%w: detached harness submission %q is already bound to a different task payload",
			taskpkg.ErrValidation,
			req.SubmissionKey,
		)
	}
	switch {
	case record.Owner == nil:
		return fmt.Errorf(
			"%w: detached harness task %q is missing owner session metadata",
			taskpkg.ErrValidation,
			record.ID,
		)
	case record.Owner.Kind != taskpkg.OwnerKindAgentSession || record.Owner.Ref != req.OwnerSessionID:
		return fmt.Errorf(
			"%w: detached harness task %q is already bound to owner %q/%q",
			taskpkg.ErrValidation,
			record.ID,
			record.Owner.Kind,
			record.Owner.Ref,
		)
	}

	current, err := decodeDetachedHarnessTaskMetadata(record.Metadata)
	if err != nil {
		return err
	}
	if current != expected {
		return fmt.Errorf(
			"%w: detached harness task %q metadata does not match submission %q",
			taskpkg.ErrValidation,
			record.ID,
			req.SubmissionKey,
		)
	}
	return nil
}

func validateDetachedHarnessRunMatch(
	run taskpkg.Run,
	req normalizedDetachedHarnessSubmitRequest,
	origin taskpkg.Origin,
	expected detachedHarnessRunMetadata,
) error {
	if run.Origin != origin {
		return fmt.Errorf(
			"%w: detached harness run %q origin %q/%q does not match submission origin %q/%q",
			taskpkg.ErrValidation,
			run.ID,
			run.Origin.Kind,
			run.Origin.Ref,
			origin.Kind,
			origin.Ref,
		)
	}
	current, err := decodeDetachedHarnessRunMetadata(run.Metadata)
	if err != nil {
		return err
	}
	current.Reentry = nil
	if current != expected {
		return fmt.Errorf(
			"%w: detached harness run %q metadata does not match submission %q",
			taskpkg.ErrValidation,
			run.ID,
			req.SubmissionKey,
		)
	}
	return nil
}

func decodeDetachedHarnessTaskMetadata(raw json.RawMessage) (detachedHarnessTaskMetadata, error) {
	var metadata detachedHarnessTaskMetadata
	if err := decodeDetachedHarnessMetadata(raw, &metadata); err != nil {
		return detachedHarnessTaskMetadata{}, fmt.Errorf(
			"%w: decode detached harness task metadata: %v",
			taskpkg.ErrValidation,
			err,
		)
	}
	if metadata.Schema != harnessDetachedMetadataSchema || metadata.Kind != harnessDetachedTaskMetadataKey {
		return detachedHarnessTaskMetadata{}, fmt.Errorf(
			"%w: detached harness task metadata has unsupported schema %q/%q",
			taskpkg.ErrValidation,
			metadata.Schema,
			metadata.Kind,
		)
	}
	return metadata, nil
}

func decodeDetachedHarnessRunMetadata(raw json.RawMessage) (detachedHarnessRunMetadata, error) {
	var metadata detachedHarnessRunMetadata
	if err := decodeDetachedHarnessMetadata(raw, &metadata); err != nil {
		return detachedHarnessRunMetadata{}, fmt.Errorf(
			"%w: decode detached harness run metadata: %v",
			taskpkg.ErrValidation,
			err,
		)
	}
	if metadata.Schema != harnessDetachedMetadataSchema || metadata.Kind != harnessDetachedRunMetadataKey {
		return detachedHarnessRunMetadata{}, fmt.Errorf(
			"%w: detached harness run metadata has unsupported schema %q/%q",
			taskpkg.ErrValidation,
			metadata.Schema,
			metadata.Kind,
		)
	}
	return metadata, nil
}

func decodeDetachedHarnessMetadata(raw json.RawMessage, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}

	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}

func maybeDecodeDetachedHarnessRunMetadata(raw json.RawMessage) (detachedHarnessRunMetadata, bool, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return detachedHarnessRunMetadata{}, false, nil
	}

	var probe struct {
		Schema string `json:"schema"`
		Kind   string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return detachedHarnessRunMetadata{}, false, nil
	}
	if strings.TrimSpace(probe.Schema) != harnessDetachedMetadataSchema ||
		strings.TrimSpace(probe.Kind) != harnessDetachedRunMetadataKey {
		return detachedHarnessRunMetadata{}, false, nil
	}

	metadata, err := decodeDetachedHarnessRunMetadata(raw)
	if err != nil {
		return detachedHarnessRunMetadata{}, false, err
	}
	return metadata, true, nil
}
