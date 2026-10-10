package globaldb

import (
	"context"
	"encoding/json"

	"github.com/compozy/compozy/internal/store"
)

// Register only new attributed admissions, inside the admission/queue transaction.
// Replays return before this boundary; historical v4 rows never acquire a watch.
func registerAdmissionReplyWatch(
	ctx context.Context,
	exec globalSQLExecutor,
	admission store.SessionPromptAdmission,
) error {
	if admission.FingerprintVersion != "session-prompt/v5" || len(admission.Origin) == 0 {
		return nil
	}
	var origin struct {
		SessionID        string `json:"session_id"`
		WorkspaceID      string `json:"workspace_id"`
		Hop              int    `json:"hop"`
		NotifyOnComplete bool   `json:"notify_on_complete"`
	}
	if err := json.Unmarshal(admission.Origin, &origin); err != nil {
		return err
	}
	if !origin.NotifyOnComplete {
		return nil
	}
	_, err := insertReplyWatchTx(ctx, exec, store.ReplyWatchRegistration{
		WorkspaceID: origin.WorkspaceID, SenderSessionID: origin.SessionID,
		TargetWorkspaceID: admission.WorkspaceID, TargetSessionID: admission.SessionID,
		MessageID: admission.MessageID, AdmissionID: admission.ID, Hop: origin.Hop, CreatedAt: admission.CreatedAt,
	})
	return err
}
