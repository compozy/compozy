package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"runtime/trace"
	"strings"
	"sync"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/heartbeat"
	"github.com/compozy/compozy/internal/modelcatalog"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
)

// Diagnostics run separately from acceptance timings, only after a failed trial.
// The trace distinguishes scheduling, GC, Go locks and system-call stalls.
func traceLoad(g *globaldb.GlobalDB, opts options, m *measurements, openMS float64) (err error) {
	if opts.runtimeTrace == "" {
		return load(g, opts, m, openMS)
	}
	file, err := os.OpenFile(opts.runtimeTrace, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return errors.Join(err, g.Close(context.Background()))
	}
	if err := trace.Start(file); err != nil {
		return errors.Join(err, file.Close(), g.Close(context.Background()))
	}
	defer func() {
		trace.Stop()
		err = errors.Join(err, file.Close())
	}()
	return load(g, opts, m, openMS)
}

func load(g *globaldb.GlobalDB, opts options, m *measurements, openMS float64) (err error) {
	sessions := make([]*sessiondb.SessionDB, 0, opts.sessions)
	closed := false
	defer func() {
		for _, s := range sessions {
			err = errors.Join(err, s.Close(context.Background()))
		}
		if !closed {
			err = errors.Join(err, g.Close(context.Background()))
		}
	}()
	for i := range opts.sessions {
		s, openErr := openSession(context.Background(), opts, i)
		if openErr != nil {
			return openErr
		}
		sessions = append(sessions, s)
	}
	warmupStart := time.Now().UTC()
	runWorkload(g, sessions, opts.warmup, m, "warmup")
	start := time.Now().UTC()
	m.active.Store(true)
	retention := runWorkload(g, sessions, opts.duration, m, "measured")
	// Include the flush and checkpoints of all accepted work in transaction and
	// file-write accounting. Never cancel accepted requests at the stop boundary.
	for _, s := range sessions {
		m.call("session_close", func() error { return s.Close(context.Background()) })
	}
	sessions = nil
	m.call("global_close", func() error { return g.Close(context.Background()) })
	closed = true
	end := time.Now().UTC()
	m.active.Store(false)
	result := m.result(start, end, openMS)
	result["sessions"] = opts.sessions
	result["history_turns"] = opts.turns
	result["warmup_start"] = warmupStart
	result["retention"] = retention
	return json.NewEncoder(os.Stdout).Encode(result)
}

func runWorkload(
	g *globaldb.GlobalDB,
	sessions []*sessiondb.SessionDB,
	duration time.Duration,
	m *measurements,
	phase string,
) store.ObservabilityRetentionSweepResult {
	var retention store.ObservabilityRetentionSweepResult
	if duration == 0 {
		return retention
	}
	until := time.Now().Add(duration)
	var workers sync.WaitGroup
	repeat := func(name string, interval time.Duration, fn func(context.Context, int) error) {
		workers.Go(func() {
			ticker := time.NewTicker(interval)
			defer ticker.Stop()
			for n := 0; time.Now().Before(until); n++ {
				m.call(name, func() error {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
					defer cancel()
					return fn(ctx, n)
				})
				if time.Now().Before(until) {
					<-ticker.C
				}
			}
		})
	}
	for i, s := range sessions {
		scheduleSessionLoad(g, s, sessionID(i), phase, repeat)
	}

	repeat("catalog_read", 50*time.Millisecond, func(ctx context.Context, _ int) error {
		page, err := g.PageSessions(
			ctx,
			store.SessionCatalogPageQuery{
				ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID},
				Sort:      "recent",
				Limit:     30,
			},
		)
		if err == nil && len(page.Sessions) == 0 {
			return errors.New("empty catalog page")
		}
		return err
	})
	repeat(
		"catalog_refresh",
		2*time.Second,
		func(ctx context.Context, n int) error { return refreshCatalog(ctx, g, n/3) },
	)
	repeat("process_observations", 2*time.Second, func(ctx context.Context, _ int) error {
		observations := make([]sessionprocess.Observation, 0, len(sessions)*3)
		for i := range len(sessions) * 3 {
			observations = append(observations, sessionprocess.Observation{
				PID:       10000 + i,
				StartedAt: fixtureTime,
				Owner: sessionprocess.Owner{
					SessionID:   sessionID(i % len(sessions)),
					WorkspaceID: workspaceID,
					ProfileID:   store.DefaultProfileID,
					Name:        "synthetic",
				},
				PGID:            10000 + i,
				FirstSeen:       fixtureTime,
				GroupObservedAt: time.Now().UTC(),
				Attribution:     "lineage",
				KernelToken:     fmt.Sprintf("synthetic-%d", i),
			})
		}
		return g.SaveProcessObservations(ctx, observations, nil)
	})
	if phase == "measured" {
		workers.Go(func() { retention = runRetention(g, m) })
	}
	workers.Wait()
	return retention
}

func runRetention(g *globaldb.GlobalDB, m *measurements) (result store.ObservabilityRetentionSweepResult) {
	m.call("retention_sweep", func() error {
		// A sweep may contain many bounded commits. Its overall deadline is
		// separate from the two-second interactive request deadline.
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		var err error
		result, err = g.SweepObservability(ctx, time.Date(2026, 10, 26, 6, 0, 0, 0, time.UTC))
		return err
	})
	return result
}

func refreshCatalog(ctx context.Context, g *globaldb.GlobalDB, generation int) error {
	now := time.Now().UTC()
	rows := make([]modelcatalog.ModelRow, 1500)
	for i := range rows {
		rows[i] = modelcatalog.ModelRow{
			SourceID:      "models_dev",
			ProviderID:    fixtureProvider,
			ModelID:       fmt.Sprintf("synthetic-%04d", i),
			SourceKind:    modelcatalog.SourceKindModelsDev,
			Priority:      50,
			Available:     new(true),
			RefreshedAt:   now,
			DisplayName:   fmt.Sprintf("Synthetic %d", i),
			ContextWindow: new(int64(200000)),
			SupportsTools: new(true),
			ReasoningEfforts: []modelcatalog.ReasoningEffort{
				modelcatalog.ReasoningEffortLow,
				modelcatalog.ReasoningEffortHigh,
			},
		}
	}
	rows[0].DisplayName = fmt.Sprintf("Generation %d", generation)
	return g.ReplaceSourceRows(
		ctx,
		modelcatalog.GlobalCatalogExecutionContext(),
		"models_dev",
		fixtureProvider,
		rows,
		modelcatalog.SourceStatus{
			SourceID:     "models_dev",
			ProviderID:   fixtureProvider,
			SourceKind:   modelcatalog.SourceKindModelsDev,
			Priority:     50,
			LastRefresh:  now,
			LastSuccess:  now,
			RefreshState: modelcatalog.RefreshStateSucceeded,
		},
	)
}

func scheduleSessionLoad(g *globaldb.GlobalDB, s *sessiondb.SessionDB, id, phase string,
	repeat func(string, time.Duration, func(context.Context, int) error)) {
	repeat("transcript_write", 50*time.Millisecond, func(ctx context.Context, n int) error {
		e, err := event(
			fmt.Sprintf("%s-%d", phase, n),
			phase,
			acp.EventTypeAgentMessage,
			strings.Repeat("stream ", 9),
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return s.Record(ctx, e)
	})
	repeat("transcript_read", 200*time.Millisecond, func(ctx context.Context, _ int) error {
		page, err := s.TranscriptPage(ctx, transcript.PageQuery{Limit: 60})
		if err == nil && len(page.Entries) == 0 {
			return errors.New("empty transcript page")
		}
		return err
	})
	repeat("health", 250*time.Millisecond, func(ctx context.Context, _ int) error {
		now := time.Now().UTC()
		_, err := g.UpsertSessionHealth(
			ctx,
			heartbeat.SessionHealth{
				SessionID:           id,
				WorkspaceID:         workspaceID,
				AgentName:           fixtureAgent,
				State:               heartbeat.SessionHealthStatePrompting,
				Health:              heartbeat.SessionHealthHealthy,
				ActivePrompt:        true,
				Attachable:          true,
				IneligibilityReason: string(heartbeat.SessionHealthReasonPromptActive),
				LastActivityAt:      now,
				LastPresenceAt:      now,
				UpdatedAt:           now,
			},
		)
		return err
	})
	repeat("summary", 500*time.Millisecond, func(ctx context.Context, n int) error {
		summary := store.EventSummary{
			ID:          fmt.Sprintf("%s-%s-summary-%d", phase, id, n),
			ProfileID:   store.DefaultProfileID,
			SessionID:   id,
			WorkspaceID: workspaceID,
			AgentName:   fixtureAgent,
			Provider:    fixtureProvider,
			Type:        "agent_message",
			Outcome:     "info",
			Summary:     "synthetic stream",
			Timestamp:   time.Now().UTC(),
		}
		summary.SetContent([]byte(`{"message":"` + strings.Repeat("synthetic ", 36) + `"}`))
		return g.WriteEventSummary(ctx, summary)
	})
	repeat("interactive_state", 500*time.Millisecond, func(ctx context.Context, _ int) error {
		return g.UpdateSessionState(
			ctx,
			store.SessionStateUpdate{ID: id, State: "active", UpdatedAt: time.Now().UTC()},
		)
	})
}
