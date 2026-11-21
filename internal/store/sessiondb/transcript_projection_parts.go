package sessiondb

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	jsonv2 "encoding/json/v2"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/store/sessiondb/sqlcgen"
	"github.com/compozy/compozy/internal/transcript"
)

// Field numbers are persisted, and the header must sort before its fields.
const (
	partHeaderField = iota
	partInputField
	partRawInputField
	partOutputField
	partDataField
	partTitleField
	partErrorField
	partTextField
)

func prepareProjectionParts(
	key string, sequence int64, changes []transcript.ProjectedPartChange,
) ([]sqlcgen.UpsertTranscriptEntryPartParams, error) {
	rows := make([]sqlcgen.UpsertTranscriptEntryPartParams, 0, len(changes))
	for index := range changes {
		change := &changes[index]
		var err error
		rows, err = appendProjectionPartRows(rows, key, sequence, change)
		if err != nil {
			return nil, fmt.Errorf("store: encode transcript entry %q part %d: %w", key, change.Index, err)
		}
	}
	return rows, nil
}

func appendProjectionPartRows(
	rows []sqlcgen.UpsertTranscriptEntryPartParams, key string, sequence int64, change *transcript.ProjectedPartChange,
) ([]sqlcgen.UpsertTranscriptEntryPartParams, error) {
	add := func(field, seq int64, content string) {
		rows = append(rows, sqlcgen.UpsertTranscriptEntryPartParams{
			EntryKey: key, PartIndex: int64(change.Index), Field: field, Sequence: seq, Content: content,
		})
	}
	before, part := change.Before, change.Part
	for _, value := range []struct {
		field  int64
		before json.RawMessage
		part   json.RawMessage
	}{
		{partInputField, before.Input, part.Input},
		{partRawInputField, before.RawInput, part.RawInput},
		{partOutputField, before.Output, part.Output},
		{partDataField, before.Data, part.Data},
	} {
		if bytes.Equal(value.before, value.part) {
			continue
		}
		var encoded []byte
		if len(value.part) > 0 {
			var err error
			encoded, err = jsonv2.Marshal(value.part, json.DefaultOptionsV1())
			if err != nil {
				return nil, err
			}
		}
		add(value.field, 0, string(encoded))
	}
	if before.Title != part.Title {
		add(partTitleField, 0, part.Title)
	}
	if before.ErrorText != part.ErrorText {
		add(partErrorField, 0, part.ErrorText)
	}
	part.Input, part.RawInput, part.Output, part.Data = nil, nil, nil, nil
	part.Title, part.ErrorText = "", ""
	before.Input, before.RawInput, before.Output, before.Data = nil, nil, nil, nil
	before.Title, before.ErrorText = "", ""
	header, err := jsonv2.Marshal(part, json.DefaultOptionsV1())
	if err != nil {
		return nil, err
	}
	previous, err := jsonv2.Marshal(before, json.DefaultOptionsV1())
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(previous, header) {
		add(partHeaderField, 0, string(header))
	}
	if change.Text != "" {
		add(partTextField, sequence, change.Text)
	}
	return rows, nil
}

type projectedMessage struct {
	key      string
	raw      sql.NullString
	parts    int64
	complete bool
}

func (m projectedMessage) decode(ctx context.Context, queries *sqlcgen.Queries) (transcript.UIMessage, error) {
	var message transcript.UIMessage
	if !m.raw.Valid {
		return message, fmt.Errorf("%w: missing transcript message %q", transcript.ErrProjectionCorrupt, m.key)
	}
	if err := jsonv2.Unmarshal([]byte(m.raw.String), &message, json.DefaultOptionsV1()); err != nil {
		return message, fmt.Errorf("%w: decode message %q: %v", transcript.ErrProjectionCorrupt, m.key, err)
	}
	if m.parts == 0 {
		return message, nil
	}
	rows, err := queries.ListTranscriptEntryParts(ctx, m.key)
	if err != nil {
		return message, fmt.Errorf("store: read transcript entry %q parts: %w", m.key, err)
	}
	message.Parts, err = assembleProjectionParts(rows, m.parts, m.complete)
	if err != nil {
		return message, fmt.Errorf("%w: decode message %q parts: %v", transcript.ErrProjectionCorrupt, m.key, err)
	}
	return message, nil
}

func assembleProjectionParts(
	rows []sqlcgen.ListTranscriptEntryPartsRow, count int64, complete bool,
) ([]transcript.UIMessagePart, error) {
	parts := make([]transcript.UIMessagePart, 0, min(int64(len(rows)), max(0, count)))
	var text strings.Builder
	finishPart := func() {
		if len(parts) > 0 {
			part := &parts[len(parts)-1]
			part.Text = projectionString(text.String())
			text.Reset()
			transcript.SetProjectedStreamState(part, complete)
		}
	}
	for _, row := range rows {
		if row.Field == partHeaderField && row.PartIndex == int64(len(parts)) {
			finishPart()
			var part transcript.UIMessagePart
			if err := jsonv2.Unmarshal([]byte(row.Content), &part, json.DefaultOptionsV1()); err != nil {
				return nil, err
			}
			parts = append(parts, part)
			continue
		}
		if len(parts) == 0 || row.PartIndex != int64(len(parts)-1) {
			return nil, fmt.Errorf("missing part header at index %d", row.PartIndex)
		}
		if err := applyProjectionPartField(&parts[len(parts)-1], &text, row); err != nil {
			return nil, err
		}
	}
	finishPart()
	if int64(len(parts)) != count {
		return nil, fmt.Errorf("found %d parts, expected %d", len(parts), count)
	}
	return parts, nil
}

func applyProjectionPartField(
	part *transcript.UIMessagePart, text *strings.Builder, row sqlcgen.ListTranscriptEntryPartsRow,
) error {
	var raw json.RawMessage
	if row.Content != "" && row.Field >= partInputField && row.Field <= partDataField {
		raw = json.RawMessage(row.Content)
		if !json.Valid(raw) {
			return fmt.Errorf("invalid JSON at part %d field %d", row.PartIndex, row.Field)
		}
	}
	switch row.Field {
	case partInputField:
		part.Input = raw
	case partRawInputField:
		part.RawInput = raw
	case partOutputField:
		part.Output = raw
	case partDataField:
		part.Data = raw
	case partTitleField:
		part.Title = projectionString(row.Content)
	case partErrorField:
		part.ErrorText = projectionString(row.Content)
	case partTextField:
		text.WriteString(row.Content)
	default:
		return fmt.Errorf("unexpected field %d at part %d", row.Field, row.PartIndex)
	}
	return nil
}

// The previous JSON snapshot replaced invalid UTF-8 after joining the text chunks.
func projectionString(value string) string {
	if utf8.ValidString(value) {
		return value
	}
	return string([]rune(value))
}
