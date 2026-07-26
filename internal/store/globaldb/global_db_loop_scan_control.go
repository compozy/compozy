package globaldb

import (
	"fmt"
	"strings"

	looppkg "github.com/compozy/agh/internal/loop"
	taskpkg "github.com/compozy/agh/internal/task"
)

func applyLoopRunControlScan(run *looppkg.Run, values *loopRunScanValues) error {
	if !values.controlActorKind.Valid && !values.controlActorID.Valid && !values.controlRequested.Valid {
		return nil
	}
	if !values.controlActorKind.Valid || !values.controlActorID.Valid || !values.controlRequested.Valid {
		return fmt.Errorf("%w: loop control actor identity is incomplete", looppkg.ErrValidation)
	}
	run.ControlActor = taskpkg.ActorIdentity{
		Kind: taskpkg.ActorKind(strings.TrimSpace(values.controlActorKind.String)),
		Ref:  strings.TrimSpace(values.controlActorID.String),
	}
	requestedAt, err := parseLoopRunTimestamp(values.controlRequested.String)
	if err != nil {
		return fmt.Errorf("store: parse loop control requested_at: %w", err)
	}
	run.ControlRequestedAt = requestedAt
	return nil
}
