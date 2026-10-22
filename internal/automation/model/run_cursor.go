package model

import (
	"fmt"

	"github.com/compozy/compozy/internal/listcursor"
	"github.com/compozy/compozy/internal/store"
)

type RunCursorPosition struct {
	ID          string `json:"id"`
	Started     string `json:"started"`
	NullStarted bool   `json:"null_started"`
}

func runQueryFingerprint(query RunQuery) (string, error) {
	query.Cursor = ""
	query.Limit = 0
	return listcursor.Fingerprint(query)
}

func DecodeRunCursor(query RunQuery) (RunCursorPosition, error) {
	if query.Cursor == "" {
		return RunCursorPosition{}, nil
	}
	fingerprint, err := runQueryFingerprint(query)
	if err != nil {
		return RunCursorPosition{}, err
	}
	position, err := listcursor.Decode[RunCursorPosition](
		query.Cursor,
		1,
		"automation_runs",
		fingerprint,
		listcursor.DefaultMaxEncodedSize,
	)
	if err != nil {
		return RunCursorPosition{}, fmt.Errorf("%w: %w", ErrListCursorInvalid, err)
	}
	if position.ID == "" || (!position.NullStarted && position.Started == "") {
		return RunCursorPosition{}, ErrListCursorInvalid
	}
	return position, nil
}

func EncodeRunCursor(query RunQuery, run Run) (string, error) {
	fingerprint, err := runQueryFingerprint(query)
	if err != nil {
		return "", err
	}
	position := RunCursorPosition{ID: run.ID, NullStarted: run.StartedAt == nil}
	if run.StartedAt != nil {
		position.Started = store.FormatTimestamp(*run.StartedAt)
	}
	return listcursor.Encode(1, "automation_runs", fingerprint, position)
}
