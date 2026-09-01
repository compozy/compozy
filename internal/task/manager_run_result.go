package task

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ReadTaskRunResult returns one authorized exact-byte page from a run result.
func (m *Service) ReadTaskRunResult(
	ctx context.Context,
	runID string,
	offset int64,
	limit int64,
	actor ActorContext,
) (RunResultPage, error) {
	if err := requireReadAuthority(actor); err != nil {
		return RunResultPage{}, err
	}
	trimmedRunID := strings.TrimSpace(runID)
	if trimmedRunID == "" {
		return RunResultPage{}, fmt.Errorf("%w: task run id is required", ErrValidation)
	}
	if offset < 0 || limit < 0 || limit > MaxRunResultPageBytes {
		return RunResultPage{}, fmt.Errorf(
			"%w: %w",
			ErrValidation,
			ErrTaskRunResultInvalidRange,
		)
	}

	run, err := m.loadRun(ctx, trimmedRunID)
	if err != nil {
		return RunResultPage{}, maskRunResultReadError(err)
	}
	taskRecord, err := m.runDetailTask(ctx, run, actor)
	if err != nil {
		return RunResultPage{}, maskRunResultReadError(err)
	}
	if err := m.runReadAuthorizer.AuthorizeRunRead(ctx, actor, run, taskRecord); err != nil {
		return RunResultPage{}, maskRunResultReadError(err)
	}
	if result := run.ResultValue(); len(result) > 0 {
		return PageRunResult(run.ID, "", result, offset, limit)
	}
	resultRef := run.ResultReference()
	resultBytes := run.ResultByteCount()
	if strings.TrimSpace(resultRef) == "" || resultBytes <= 0 {
		return RunResultPage{}, ErrTaskRunResultNotFound
	}

	page, err := m.store.ReadTaskRunResultPage(ctx, run.ID, offset, limit)
	if err != nil {
		return RunResultPage{}, err
	}
	if page.RunID != run.ID || page.ResultRef != resultRef || page.TotalBytes != resultBytes {
		return RunResultPage{}, fmt.Errorf("%w: task run result descriptor changed", ErrTaskRunResultCorrupt)
	}
	return page, nil
}

func maskRunResultReadError(err error) error {
	if errors.Is(err, ErrTaskRunNotFound) || errors.Is(err, ErrTaskNotFound) ||
		errors.Is(err, ErrPermissionDenied) {
		return ErrTaskRunResultNotFound
	}
	return err
}
