package goal

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop"
)

const (
	// MaxReportEvidenceBytes is the maximum inline evidence accepted by goal_report.
	MaxReportEvidenceBytes = 16 * 1024
)

// ToolReportTarget pins the only active prompt identity eligible for goal_report.
type ToolReportTarget struct {
	Key                  TurnKey
	ExpectedControlEpoch int64
	ExpectedBindingEpoch int64
	PromptID             string
	OriginSessionID      string
	BoundSessionID       string
}

// Validate enforces a complete prompt/control/binding identity.
func (t ToolReportTarget) Validate() error {
	if err := t.Key.Validate(); err != nil {
		return err
	}
	if t.ExpectedControlEpoch < 1 || t.ExpectedBindingEpoch < 1 ||
		strings.TrimSpace(t.PromptID) == "" || strings.TrimSpace(t.OriginSessionID) == "" ||
		strings.TrimSpace(t.BoundSessionID) == "" {
		return fmt.Errorf("%w: Goal tool report target is incomplete", loop.ErrValidation)
	}
	return nil
}

// RecordToolReportRequest records one prompt-bound report and its inline evidence atomically.
type RecordToolReportRequest struct {
	Target    ToolReportTarget
	Status    string
	Evidence  string
	ActorKind string
	ActorID   string
}

// ToolStore is the durable Goal subset required by session-scoped native tools.
type ToolStore interface {
	FindGoalReportTarget(
		context.Context,
		loop.WorkspaceID,
		string,
	) (ToolReportTarget, bool, error)
	ResolveActiveGoalOriginAlias(context.Context, loop.WorkspaceID, string) (string, bool, error)
	RecordGoalReport(context.Context, RecordToolReportRequest) (ReportIntent, error)
}
