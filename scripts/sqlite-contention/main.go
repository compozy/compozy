// Command sqlite-contention measures the same public store workload at two commits.
// Keep its imports and calls compatible with the documented baseline.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/acp"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/store/globaldb"
	"github.com/compozy/compozy/internal/store/sessiondb"
	"github.com/compozy/compozy/internal/transcript"
	"github.com/compozy/compozy/internal/workspace"
)

const (
	workspaceID     = "contention-workspace"
	fixtureAgent    = "coder"
	fixtureProvider = "claude"
	openMetric      = "open_ms"
)

var fixtureTime = time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)

type options struct {
	mode, root       string
	runtimeTrace     string
	sessions, turns  int
	duration, warmup time.Duration
}

func main() {
	var opts options
	flag.StringVar(&opts.mode, "mode", "load", "seed, inspect, or load")
	flag.StringVar(&opts.root, "root", "", "isolated database family root")
	flag.StringVar(&opts.runtimeTrace, "runtime-trace", "", "optional diagnostic Go execution trace")
	flag.IntVar(&opts.sessions, "sessions", 30, "number of streaming sessions")
	flag.IntVar(&opts.turns, "turns", 1000, "completed turns per session before measurement")
	flag.DurationVar(&opts.duration, "duration", 30*time.Second, "measured workload duration")
	flag.DurationVar(&opts.warmup, "warmup", 5*time.Second, "unmeasured warmup")
	flag.Parse()
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(opts); err != nil {
		slog.Error("experiment failed", "error", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	if opts.root == "" || opts.sessions < 1 || opts.turns < 1 || opts.duration <= 0 || opts.warmup < 0 {
		return errors.New("invalid experiment options")
	}
	root, err := filepath.Abs(opts.root)
	if err != nil {
		return err
	}
	opts.root = root
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	metrics := newMeasurements(filepath.Join(root, "compozy.db"))
	if err := metrics.install(); err != nil {
		return err
	}
	started := time.Now()
	g, err := globaldb.OpenGlobalDB(context.Background(), filepath.Join(root, "compozy.db"))
	if err != nil {
		return err
	}
	openMS := float64(time.Since(started)) / float64(time.Millisecond)
	var result any
	switch opts.mode {
	case "seed":
		err = seed(context.Background(), g, opts)
		result = map[string]any{openMetric: openMS, "sessions": opts.sessions, "turns": opts.turns}
	case "inspect":
		result, err = inspect(context.Background(), g, openMS)
	case "load":
		return traceLoad(g, opts, metrics, openMS)
	default:
		err = fmt.Errorf("unknown mode %q", opts.mode)
	}
	err = errors.Join(err, g.Close(context.Background()))
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(result)
}

func sessionID(i int) string { return fmt.Sprintf("contention-session-%02d", i) }
func sessionPath(opts options, i int) string {
	return filepath.Join(opts.root, "sessions", sessionID(i), "events.db")
}
func openSession(ctx context.Context, opts options, i int) (*sessiondb.SessionDB, error) {
	return sessiondb.OpenSessionDB(
		ctx,
		store.SessionDBOwner{SessionID: sessionID(i), WorkspaceID: workspaceID},
		sessionPath(opts, i),
	)
}
func event(id, turn, kind, text string, stamp time.Time) (store.SessionEvent, error) {
	payload, err := transcript.MarshalAgentEvent(acp.AgentEvent{Type: kind, TurnID: turn, Text: text, Timestamp: stamp})
	return store.SessionEvent{
		ID:        id,
		TurnID:    turn,
		Type:      kind,
		AgentName: fixtureAgent,
		Content:   payload,
		Timestamp: stamp,
	}, err
}
func seed(ctx context.Context, g *globaldb.GlobalDB, opts options) error {
	if err := g.InsertWorkspace(
		ctx,
		workspace.Workspace{
			ID:        workspaceID,
			RootDir:   opts.root,
			Name:      "Contention fixture",
			CreatedAt: fixtureTime,
			UpdatedAt: fixtureTime,
		},
	); err != nil {
		return err
	}
	for i := range opts.sessions {
		id := sessionID(i)
		if err := g.RegisterSession(
			ctx,
			&store.SessionInfo{
				ID:            id,
				ProfileID:     store.DefaultProfileID,
				WorkspaceID:   workspaceID,
				AgentName:     fixtureAgent,
				Provider:      fixtureProvider,
				State:         "active",
				RuntimeStatus: store.SessionRuntimeUnbound,
				CreatedAt:     fixtureTime,
				UpdatedAt:     fixtureTime,
			},
		); err != nil {
			return err
		}
		s, err := openSession(ctx, opts, i)
		if err != nil {
			return err
		}
		err = seedTranscript(ctx, s, opts.turns)
		if err := errors.Join(err, s.Close(ctx)); err != nil {
			return err
		}
		summary := store.EventSummary{
			ID:          fmt.Sprintf("seed-summary-%02d", i),
			ProfileID:   store.DefaultProfileID,
			SessionID:   id,
			WorkspaceID: workspaceID,
			Type:        "agent_message",
			AgentName:   fixtureAgent,
			Provider:    fixtureProvider,
			Outcome:     "info",
			Summary:     "seed",
			Timestamp:   fixtureTime,
		}
		summary.SetContent([]byte(`{"message":"synthetic seed"}`))
		if err := g.WriteEventSummary(ctx, summary); err != nil {
			return err
		}
	}
	return refreshCatalog(ctx, g, 0)
}
func seedTranscript(ctx context.Context, s *sessiondb.SessionDB, turns int) error {
	batch := make([]store.SessionEvent, 0, 3*turns)
	for i := range turns {
		turn := fmt.Sprintf("seed-turn-%05d", i)
		parts := []struct{ kind, text string }{
			{acp.EventTypeUserMessage, "Synthetic question"},
			{acp.EventTypeAgentMessage, strings.Repeat("seed answer ", 24)},
			{acp.EventTypeDone, ""},
		}
		for j, part := range parts {
			e, err := event(
				fmt.Sprintf("seed-%05d-%d", i, j),
				turn,
				part.kind,
				part.text,
				fixtureTime.Add(time.Duration(i)*time.Second),
			)
			if err != nil {
				return err
			}
			batch = append(batch, e)
		}
	}
	_, err := s.RecordPersistedBatch(ctx, batch)
	return err
}

func inspect(ctx context.Context, g *globaldb.GlobalDB, openMS float64) (any, error) {
	queries := []store.EventSummaryQuery{
		{ReadScope: store.ReadScope{AllProfiles: true}, Limit: 100},
		{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, Limit: 100, SequenceOrder: true},
		{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, WorkspaceID: workspaceID, Limit: 100},
		{ReadScope: store.ReadScope{ProfileID: store.DefaultProfileID}, SessionID: sessionID(7), Limit: 100},
		{ReadScope: store.ReadScope{AllProfiles: true}, Provider: fixtureProvider, ErrorOnly: true, Limit: 100},
		{ReadScope: store.ReadScope{AllProfiles: true}, TaskID: "task-17", Limit: 100},
		{ReadScope: store.ReadScope{AllProfiles: true}, RunID: "run-23", Limit: 100},
	}
	results := make([][]store.EventSummary, 0, len(queries)+1)
	var maxSeq int64
	for _, q := range queries {
		rows, err := g.ListEventSummaries(ctx, q)
		if err != nil {
			return nil, err
		}
		results = append(results, rows)
		for _, r := range rows {
			maxSeq = max(maxSeq, r.Sequence)
		}
	}
	forward, err := g.ListEventSummaries(
		ctx,
		store.EventSummaryQuery{
			ReadScope:     store.ReadScope{AllProfiles: true},
			Limit:         100,
			SequenceOrder: true,
			Forward:       true,
			AfterSequence: max(0, maxSeq-1000),
		},
	)
	if err != nil {
		return nil, err
	}
	results = append(results, forward)
	return map[string]any{openMetric: openMS, "observe": results}, nil
}
