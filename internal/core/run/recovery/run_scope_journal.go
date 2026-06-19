package recovery

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/journal"
	"github.com/compozy/compozy/pkg/compozy/events"
)

type runScopeJournalSetter interface {
	SetRunJournal(*journal.Journal)
}

// RefreshRunScopeJournal replaces the scope journal with a fresh append handle.
func RefreshRunScopeJournal(ctx context.Context, scope model.RunScope) error {
	if scope == nil {
		return errors.New("refresh run journal: missing run scope")
	}
	setter, ok := scope.(runScopeJournalSetter)
	if !ok {
		return fmt.Errorf("refresh run journal: unsupported run scope %T", scope)
	}
	artifacts := scope.RunArtifacts()
	if strings.TrimSpace(artifacts.EventsPath) == "" {
		return errors.New("refresh run journal: missing events path")
	}
	if err := closeRunScopeJournal(ctx, scope.RunJournal()); err != nil {
		return err
	}
	runJournal, err := journal.Open(artifacts.EventsPath, scope.RunEventBus(), 0)
	if err != nil {
		return fmt.Errorf("refresh run journal: open %s: %w", artifacts.EventsPath, err)
	}
	setter.SetRunJournal(runJournal)
	return nil
}

func closeRunScopeJournal(ctx context.Context, runJournal *journal.Journal) error {
	if runJournal == nil {
		return nil
	}
	closeCtx := ctx
	if closeCtx == nil {
		closeCtx = context.Background()
	}
	if _, ok := closeCtx.Deadline(); !ok {
		var cancel context.CancelFunc
		closeCtx, cancel = context.WithTimeout(closeCtx, time.Second)
		defer cancel()
	}
	if err := runJournal.Close(closeCtx); err != nil {
		return fmt.Errorf("refresh run journal: close existing journal: %w", err)
	}
	return nil
}

// NewRunScopeEventSink creates an event sink backed by the scope journal.
func NewRunScopeEventSink(scope model.RunScope) EventSink {
	if scope == nil {
		return nil
	}
	return runScopeEventSink{scope: scope}
}

type runScopeEventSink struct {
	scope model.RunScope
}

func (s runScopeEventSink) Submit(ctx context.Context, event events.Event) error {
	if s.scope == nil {
		return nil
	}
	if err := RefreshRunScopeJournal(ctx, s.scope); err != nil {
		return err
	}
	runJournal := s.scope.RunJournal()
	if runJournal == nil {
		return errors.New("submit recovery event: missing run journal")
	}
	return runJournal.Submit(ctx, event)
}
