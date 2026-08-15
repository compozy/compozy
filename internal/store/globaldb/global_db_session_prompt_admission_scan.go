package globaldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb/sqlcgen"
)

func sessionPromptAdmissionFromGenerated(
	row *sqlcgen.SessionPromptAdmission,
) (store.SessionPromptAdmission, error) {
	admission := store.SessionPromptAdmission{
		ID:                 strings.TrimSpace(row.ID),
		WorkspaceID:        strings.TrimSpace(row.WorkspaceID),
		SessionID:          strings.TrimSpace(row.SessionID),
		MessageID:          strings.TrimSpace(row.MessageID),
		IdempotencyKey:     strings.TrimSpace(row.IdempotencyKey),
		Operation:          strings.TrimSpace(row.Operation),
		FingerprintVersion: strings.TrimSpace(row.FingerprintVersion),
		RequestFingerprint: strings.TrimSpace(row.RequestFingerprint),
		State:              strings.TrimSpace(row.State),
		Mode:               strings.TrimSpace(row.Mode),
		AuthoredText:       row.AuthoredText,
		Runtime: store.SessionInputRuntime{
			Provider:        strings.TrimSpace(row.RuntimeProvider),
			Model:           strings.TrimSpace(row.RuntimeModel),
			ReasoningEffort: strings.TrimSpace(row.RuntimeReasoningEffort),
			Speed:           strings.TrimSpace(row.RuntimeSpeed),
		},
		TurnID:              strings.TrimSpace(row.TurnID),
		EventID:             strings.TrimSpace(row.EventID),
		IndeterminateReason: strings.TrimSpace(row.IndeterminateReason),
	}
	if err := json.Unmarshal([]byte(row.SkillInvocationsJson), &admission.SkillInvocations); err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf(
			"store: decode session prompt admission skill invocations: %w",
			err,
		)
	}
	attachments, err := decodeSessionInputAttachments(row.AttachmentsJson)
	if err != nil {
		return store.SessionPromptAdmission{}, err
	}
	admission.Attachments = attachments
	if row.ResultJson.Valid {
		var result store.SessionPromptAdmissionResult
		if err := json.Unmarshal([]byte(row.ResultJson.String), &result); err != nil {
			return store.SessionPromptAdmission{}, fmt.Errorf(
				"store: decode session prompt admission result: %w",
				err,
			)
		}
		admission.Result = &result
	}
	createdAt, err := store.ParseTimestamp(row.CreatedAt)
	if err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf(
			"store: parse session prompt admission created_at: %w",
			err,
		)
	}
	updatedAt, err := store.ParseTimestamp(row.UpdatedAt)
	if err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf(
			"store: parse session prompt admission updated_at: %w",
			err,
		)
	}
	admission.CreatedAt = createdAt
	admission.UpdatedAt = updatedAt
	admission.DispatchCommittedAt, err = parsePromptAdmissionTime(row.DispatchCommittedAt)
	if err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf(
			"store: parse session prompt admission dispatch_committed_at: %w",
			err,
		)
	}
	admission.CompletedAt, err = parsePromptAdmissionTime(row.CompletedAt)
	if err != nil {
		return store.SessionPromptAdmission{}, fmt.Errorf(
			"store: parse session prompt admission completed_at: %w",
			err,
		)
	}
	return admission, nil
}

func parsePromptAdmissionTime(value sql.NullString) (*time.Time, error) {
	if !value.Valid || strings.TrimSpace(value.String) == "" {
		return nil, nil
	}
	parsed, err := store.ParseTimestamp(value.String)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}
