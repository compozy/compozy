package ui

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/core/contentconv"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/core/run/internal/runshared"
	eventspkg "github.com/compozy/compozy/pkg/compozy/events"
	"github.com/compozy/compozy/pkg/compozy/events/kinds"

	tea "charm.land/bubbletea/v2"
)

func BenchmarkPrepareDispatchBatchSessionBurst(b *testing.B) {
	textBlock, err := model.NewContentBlock(model.TextBlock{Text: "chunk"})
	if err != nil {
		b.Fatalf("model.NewContentBlock: %v", err)
	}

	inputs := make([]any, 0, 258)
	inputs = append(inputs, benchmarkRuntimeEvent(b,
		eventspkg.EventKindJobStarted,
		kinds.JobStartedPayload{JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1}},
	))
	for i := 0; i < 128; i++ {
		update, err := contentconv.PublicSessionUpdate(model.SessionUpdate{
			Kind:   model.UpdateKindAgentMessageChunk,
			Blocks: []model.ContentBlock{textBlock},
			Status: model.StatusRunning,
		})
		if err != nil {
			b.Fatalf("contentconv.PublicSessionUpdate: %v", err)
		}
		inputs = append(inputs,
			benchmarkRuntimeEvent(b, eventspkg.EventKindSessionUpdate, kinds.SessionUpdatePayload{
				Index:  0,
				Update: update,
			}),
			benchmarkRuntimeEvent(b, eventspkg.EventKindUsageUpdated, kinds.UsageUpdatedPayload{
				Index: 0,
				Usage: kinds.Usage{InputTokens: 1, OutputTokens: 1, TotalTokens: 2},
			}),
		)
	}
	inputs = append(inputs, benchmarkRuntimeEvent(
		b,
		eventspkg.EventKindJobCompleted,
		kinds.JobCompletedPayload{
			JobAttemptInfo: kinds.JobAttemptInfo{Index: 0, Attempt: 1, MaxAttempts: 1},
			ExitCode:       0,
		},
	))

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctrl := &uiController{translator: newUIEventTranslator()}
		_ = ctrl.prepareDispatchBatch(inputs)
	}
}

func BenchmarkRefreshSidebarContentCachedRows(b *testing.B) {
	mdl := newUIModel(32)
	mdl.cfg = &config{}
	mdl.handleWindowSize(tea.WindowSizeMsg{Width: 140, Height: 40})
	for i := 0; i < 32; i++ {
		mdl.handleJobQueued(&jobQueuedMsg{
			Index:     i,
			CodeFile:  "task.md",
			CodeFiles: []string{"task.md"},
			Issues:    2,
			SafeName:  "task",
			OutLog:    "task.out.log",
			ErrLog:    "task.err.log",
			OutBuffer: runshared.NewLineBuffer(0),
			ErrBuffer: runshared.NewLineBuffer(0),
		})
	}
	mdl.refreshSidebarContent()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mdl.refreshSidebarContent()
	}
}

func benchmarkRuntimeEvent(b *testing.B, kind eventspkg.EventKind, payload any) eventspkg.Event {
	b.Helper()

	raw, err := json.Marshal(payload)
	if err != nil {
		b.Fatalf("json.Marshal(%T): %v", payload, err)
	}
	return eventspkg.Event{
		SchemaVersion: eventspkg.SchemaVersion,
		RunID:         "ui-benchmark",
		Timestamp:     time.Unix(0, 0).UTC(),
		Kind:          kind,
		Payload:       raw,
	}
}
