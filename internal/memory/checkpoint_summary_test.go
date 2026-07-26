package memory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	memcontract "github.com/compozy/agh/internal/memory/contract"
	"github.com/compozy/agh/internal/testutil"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestCheckpointSummaryServiceUpdate(t *testing.T) {
	t.Parallel()

	t.Run("Should normalize a resolved registration id to the stable workspace identity", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		const registrationID = "ws_runtime_registration"
		env.resolver.resolved.ID = registrationID
		record := env.sessionEndRecord("sess-registration", "selected SQLite")
		record.WorkspaceID = registrationID
		summarizer := &checkpointSummarizerStub{outputs: []string{
			checkpointSummaryFixture("The session selected SQLite."),
		}}
		service := NewCheckpointSummaryService(env.store, env.resolver, summarizer)

		if _, err := service.Update(testutil.Context(t), record); err != nil {
			t.Fatalf("Update(registration id) error = %v", err)
		}
		if got := summarizer.requests[0].WorkspaceID; got != env.workspaceID {
			t.Fatalf("summarizer workspace id = %q, want stable identity %q", got, env.workspaceID)
		}
	})

	t.Run("Should update one workspace record and consume the prior summary", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		summarizer := &checkpointSummarizerStub{outputs: []string{
			checkpointSummaryFixture("The first session selected SQLite."),
			checkpointSummaryFixture("The first session selected SQLite. The second session added WAL mode."),
		}}
		service := NewCheckpointSummaryService(
			env.store,
			env.resolver,
			summarizer,
			WithCheckpointSummaryClock(func() time.Time { return env.now }),
		)

		first, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-1", "selected SQLite"))
		if err != nil {
			t.Fatalf("Update(first) error = %v", err)
		}
		second, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-2", "enabled WAL mode"))
		if err != nil {
			t.Fatalf("Update(second) error = %v", err)
		}

		if first.Decision.Op != memcontract.OpAdd {
			t.Fatalf("first decision op = %q, want add", first.Decision.Op)
		}
		if second.Decision.Op != memcontract.OpUpdate {
			t.Fatalf("second decision op = %q, want update", second.Decision.Op)
		}
		if got := len(summarizer.requests); got != 2 {
			t.Fatalf("summarizer requests = %d, want 2", got)
		}
		if got := summarizer.requests[0].PreviousSummary; got != "" {
			t.Fatalf("first previous summary = %q, want empty", got)
		}
		if got := summarizer.requests[1].PreviousSummary; !strings.Contains(got, "selected SQLite") {
			t.Fatalf("second previous summary = %q, want first-session fact", got)
		}

		workspaceStore := env.store.ForWorkspace(env.workspaceRoot)
		headers, err := workspaceStore.List(memcontract.ScopeWorkspace)
		if err != nil {
			t.Fatalf("List(workspace) error = %v", err)
		}
		if got := checkpointHeaderCount(headers); got != 1 {
			t.Fatalf("checkpoint summary headers = %d, want 1", got)
		}
		content, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(checkpoint) error = %v", err)
		}
		if !strings.Contains(string(content), "added WAL mode") {
			t.Fatalf("checkpoint content = %q, want second-session fact", content)
		}
		_, header, err := loadCheckpointSummary(workspaceStore)
		if err != nil {
			t.Fatalf("loadCheckpointSummary() error = %v", err)
		}
		if header == nil || header.Provenance == nil ||
			len(header.Provenance.SourceSessionIDs) != 2 ||
			header.Provenance.SourceSessionIDs[0] != "sess-1" ||
			header.Provenance.SourceSessionIDs[1] != "sess-2" {
			t.Fatalf("checkpoint provenance = %#v, want ordered source sessions", header)
		}
	})

	t.Run("Should leave the checkpoint unchanged when the workspace role is disabled", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		service := NewCheckpointSummaryService(
			env.store,
			env.resolver,
			&checkpointSummarizerStub{errAt: map[int]error{0: ErrCheckpointSummaryDisabled}},
		)

		result, err := service.Update(
			testutil.Context(t),
			env.sessionEndRecord("sess-disabled", "must not persist"),
		)
		if err != nil {
			t.Fatalf("Update(disabled role) error = %v", err)
		}
		if result.Applied || result.Decision.ID != "" {
			t.Fatalf("Update(disabled role) = %#v, want no-op", result)
		}
		exists, err := env.store.ForWorkspace(env.workspaceRoot).Exists(
			memcontract.ScopeWorkspace,
			CheckpointSummaryFilename,
		)
		if err != nil {
			t.Fatalf("Exists(checkpoint) error = %v", err)
		}
		if exists {
			t.Fatal("checkpoint exists after disabled-role update")
		}
	})

	t.Run("Should restore the prior checkpoint through the decision WAL", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		service := NewCheckpointSummaryService(
			env.store,
			env.resolver,
			&checkpointSummarizerStub{outputs: []string{
				checkpointSummaryFixture("Prior checkpoint fact."),
				checkpointSummaryFixture("Replacement checkpoint fact."),
			}},
		)
		if _, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-1", "prior")); err != nil {
			t.Fatalf("Update(first) error = %v", err)
		}
		second, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-2", "replacement"))
		if err != nil {
			t.Fatalf("Update(second) error = %v", err)
		}

		reverted, err := env.store.ForWorkspace(env.workspaceRoot).RevertDecision(
			testutil.Context(t),
			second.Decision.ID,
		)
		if err != nil {
			t.Fatalf("RevertDecision() error = %v", err)
		}
		if !reverted.Reverted {
			t.Fatal("RevertDecision().Reverted = false, want true")
		}
		content, err := env.store.ForWorkspace(env.workspaceRoot).Read(
			memcontract.ScopeWorkspace,
			CheckpointSummaryFilename,
		)
		if err != nil {
			t.Fatalf("Read(reverted checkpoint) error = %v", err)
		}
		if !strings.Contains(string(content), "Prior checkpoint fact.") ||
			strings.Contains(string(content), "Replacement checkpoint fact.") {
			t.Fatalf("reverted checkpoint = %q, want only prior fact", content)
		}
	})

	t.Run("Should preserve the prior checkpoint when summarization fails", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		providerErr := errors.New("summary provider unavailable")
		summarizer := &checkpointSummarizerStub{
			outputs: []string{checkpointSummaryFixture("Stable checkpoint fact.")},
			errAt:   map[int]error{1: providerErr},
		}
		service := NewCheckpointSummaryService(env.store, env.resolver, summarizer)
		if _, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-1", "stable")); err != nil {
			t.Fatalf("Update(first) error = %v", err)
		}
		workspaceStore := env.store.ForWorkspace(env.workspaceRoot)
		before, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(before failure) error = %v", err)
		}

		if _, err := service.Update(
			testutil.Context(t),
			env.sessionEndRecord("sess-2", "must not persist"),
		); !errors.Is(err, providerErr) {
			t.Fatalf("Update(provider failure) error = %v, want %v", err, providerErr)
		}
		after, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(after failure) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("checkpoint changed after provider failure\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("Should preserve the prior checkpoint when generated output is rejected", func(t *testing.T) {
		t.Parallel()

		for _, tc := range []struct {
			name    string
			output  string
			wantErr string
		}{
			{
				name:    "missing schema",
				output:  "## Goal\nMalformed replacement.",
				wantErr: "must begin with",
			},
			{
				name:    "duplicate heading",
				output:  duplicateCheckpointSummaryHeadingFixture(),
				wantErr: "level-two headings",
			},
			{
				name:    "oversized document",
				output:  checkpointSummaryFixture(strings.Repeat("x", 25_000)),
				wantErr: "maximum is",
			},
			{
				name:    "raw claim token",
				output:  checkpointSummaryFixture("Leaked agh_claim_checkpoint-secret."),
				wantErr: "raw claim token",
			},
		} {
			t.Run("Should reject "+tc.name, func(t *testing.T) {
				t.Parallel()

				env := newCheckpointSummaryTestEnv(t)
				service := NewCheckpointSummaryService(
					env.store,
					env.resolver,
					&checkpointSummarizerStub{outputs: []string{
						checkpointSummaryFixture("Stable checkpoint fact."),
						tc.output,
					}},
				)
				if _, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-1", "stable")); err != nil {
					t.Fatalf("Update(first) error = %v", err)
				}
				workspaceStore := env.store.ForWorkspace(env.workspaceRoot)
				before, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
				if err != nil {
					t.Fatalf("Read(before rejection) error = %v", err)
				}
				if _, err := service.Update(
					testutil.Context(t),
					env.sessionEndRecord("sess-2", "must not persist"),
				); err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("Update(rejected output) error = %v, want %q", err, tc.wantErr)
				}
				after, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
				if err != nil {
					t.Fatalf("Read(after rejection) error = %v", err)
				}
				if !bytes.Equal(after, before) {
					t.Fatalf("checkpoint changed after rejected output\nbefore:\n%s\nafter:\n%s", before, after)
				}
			})
		}
	})
}

func TestCheckpointSummaryServiceCompactionCoverage(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the existing hint when the workspace role is disabled", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		seedService := NewCheckpointSummaryService(
			env.store,
			env.resolver,
			&checkpointSummarizerStub{outputs: []string{
				checkpointSummaryFixture("Stable checkpoint fact."),
			}},
		)
		if _, err := seedService.Update(
			testutil.Context(t),
			env.sessionEndRecord("sess-prior", "stable"),
		); err != nil {
			t.Fatalf("Update(prior) error = %v", err)
		}
		workspaceStore := env.store.ForWorkspace(env.workspaceRoot)
		before, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(before disabled compaction) error = %v", err)
		}

		service := NewCheckpointSummaryService(
			env.store,
			env.resolver,
			&checkpointSummarizerStub{errAt: map[int]error{0: ErrCheckpointSummaryDisabled}},
		)
		result, err := service.Compact(
			testutil.Context(t),
			env.preCompressRequest("sess-disabled", 1, 4, "must not persist"),
		)
		if err != nil {
			t.Fatalf("Compact(disabled role) error = %v", err)
		}
		if result.Decision.Applied || !strings.Contains(result.Hint.Markdown, "Stable checkpoint fact.") {
			t.Fatalf("Compact(disabled role) = %#v, want prior hint and no decision", result)
		}
		after, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(after disabled compaction) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("disabled compaction changed checkpoint\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})

	t.Run("Should record one idempotent covered range before archive", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		summarizer := &checkpointSummarizerStub{outputs: []string{
			checkpointSummaryFixture("Compacted cobalt context."),
			checkpointSummaryFixture("Compacted cobalt context through sequence seven."),
		}}
		service := NewCheckpointSummaryService(env.store, env.resolver, summarizer)
		request := env.preCompressRequest("sess-compact", 1, 4, "cobalt context")

		first, err := service.Compact(testutil.Context(t), request)
		if err != nil {
			t.Fatalf("Compact(first) error = %v", err)
		}
		if !strings.Contains(first.Hint.Markdown, "Compacted cobalt context.") {
			t.Fatalf("Compact(first).Hint.Markdown = %q, want compacted fact", first.Hint.Markdown)
		}
		workspaceStore := env.store.ForWorkspace(env.workspaceRoot)
		beforeRetry, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(first coverage) error = %v", err)
		}
		state, err := loadCheckpointSummaryState(workspaceStore)
		if err != nil {
			t.Fatalf("loadCheckpointSummaryState(first) error = %v", err)
		}
		if !state.coverage.covers(env.workspaceID, "sess-compact", 1, 4) {
			t.Fatalf("coverage = %#v, want session range 1..4", state.coverage)
		}

		repeated, err := service.Compact(testutil.Context(t), request)
		if err != nil {
			t.Fatalf("Compact(repeated) error = %v", err)
		}
		if repeated.Decision.Applied || len(summarizer.requests) != 1 {
			t.Fatalf(
				"Compact(repeated) = %#v with %d summarizer calls, want durable no-op",
				repeated,
				len(summarizer.requests),
			)
		}
		afterRetry, err := workspaceStore.Read(memcontract.ScopeWorkspace, CheckpointSummaryFilename)
		if err != nil {
			t.Fatalf("Read(repeated coverage) error = %v", err)
		}
		if !bytes.Equal(afterRetry, beforeRetry) {
			t.Fatalf("idempotent retry changed checkpoint\nbefore:\n%s\nafter:\n%s", beforeRetry, afterRetry)
		}

		second, err := service.Compact(
			testutil.Context(t),
			env.preCompressRequest("sess-compact", 5, 7, "later context"),
		)
		if err != nil {
			t.Fatalf("Compact(adjacent) error = %v", err)
		}
		if !second.Decision.Applied || len(summarizer.requests) != 2 {
			t.Fatalf(
				"Compact(adjacent) = %#v with %d summarizer calls, want one update",
				second,
				len(summarizer.requests),
			)
		}
		state, err = loadCheckpointSummaryState(workspaceStore)
		if err != nil {
			t.Fatalf("loadCheckpointSummaryState(adjacent) error = %v", err)
		}
		if !state.coverage.covers(env.workspaceID, "sess-compact", 1, 7) {
			t.Fatalf("coverage = %#v, want merged session range 1..7", state.coverage)
		}
	})

	t.Run("Should preserve resume coverage when its visible summary update is reverted", func(t *testing.T) {
		t.Parallel()

		env := newCheckpointSummaryTestEnv(t)
		service := NewCheckpointSummaryService(
			env.store,
			env.resolver,
			&checkpointSummarizerStub{outputs: []string{
				checkpointSummaryFixture("Visible prior fact."),
				checkpointSummaryFixture("Visible prior fact plus archived cobalt fact."),
			}},
		)
		if _, err := service.Update(testutil.Context(t), env.sessionEndRecord("sess-prior", "prior")); err != nil {
			t.Fatalf("Update(prior) error = %v", err)
		}
		compacted, err := service.Compact(
			testutil.Context(t),
			env.preCompressRequest("sess-compact", 1, 4, "archived cobalt fact"),
		)
		if err != nil {
			t.Fatalf("Compact() error = %v", err)
		}
		reverted, err := env.store.ForWorkspace(env.workspaceRoot).RevertDecision(
			testutil.Context(t),
			compacted.Decision.Decision.ID,
		)
		if err != nil {
			t.Fatalf("RevertDecision(compaction) error = %v", err)
		}
		if !reverted.Reverted {
			t.Fatal("RevertDecision(compaction).Reverted = false, want true")
		}
		state, err := loadCheckpointSummaryState(env.store.ForWorkspace(env.workspaceRoot))
		if err != nil {
			t.Fatalf("loadCheckpointSummaryState(reverted) error = %v", err)
		}
		if !strings.Contains(state.body, "Visible prior fact.") ||
			strings.Contains(state.body, "archived cobalt fact") {
			t.Fatalf("reverted visible body = %q, want prior content only", state.body)
		}
		if !state.coverage.covers(env.workspaceID, "sess-compact", 1, 4) {
			t.Fatalf("reverted coverage = %#v, want archived range preserved", state.coverage)
		}
		resume := renderCheckpointSummaryResumeSection(state, "sess-compact")
		if !strings.Contains(resume, "archived cobalt fact") {
			t.Fatalf("resume section = %q, want preserved coverage fallback", resume)
		}
	})

	t.Run("Should bound non-adjacent archived ranges while retaining the newest coverage", func(t *testing.T) {
		t.Parallel()

		coverage := checkpointCoverage{}
		for index := range checkpointSummarySourceLimit + 8 {
			sequence := int64(index*2 + 1)
			coverage = coverage.withRange(checkpointCoverageRange{
				WorkspaceID:  "ws-coverage",
				SessionID:    fmt.Sprintf("sess-%02d", index),
				FromSequence: sequence,
				ToSequence:   sequence,
			}, checkpointSummaryFixture("Coverage remains durable."))
		}
		if len(coverage.Ranges) != checkpointSummarySourceLimit {
			t.Fatalf("coverage ranges = %d, want bounded %d", len(coverage.Ranges), checkpointSummarySourceLimit)
		}
		if coverage.covers("ws-coverage", "sess-00", 1, 1) {
			t.Fatalf("coverage = %#v, want oldest archived range evicted", coverage)
		}
		if !coverage.covers("ws-coverage", "sess-39", 79, 79) {
			t.Fatalf("coverage = %#v, want newest archived range retained", coverage)
		}
	})
}

func TestRenderCheckpointSummaryPrompt(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the prior summary and render the Hermes schema", func(t *testing.T) {
		t.Parallel()

		request := CheckpointSummaryRequest{
			WorkspaceID:     "ws-alpha",
			SessionID:       "sess-2",
			PreviousSummary: checkpointSummaryFixture("Prior fact."),
			Snapshot: memcontract.TranscriptSnapshot{Messages: []memcontract.TranscriptMessage{{
				Sequence: 1,
				Role:     "user",
				Content:  "New fact.",
				At:       time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
			}}},
		}

		prompt, err := RenderCheckpointSummaryPrompt(request)
		if err != nil {
			t.Fatalf("RenderCheckpointSummaryPrompt() error = %v", err)
		}
		for _, want := range []string{
			"Prior fact.",
			"New fact.",
			checkpointSummaryFirstHeading,
			checkpointSummaryLastHeading,
			"PRESERVE all existing information that is still relevant",
		} {
			if !strings.Contains(prompt, want) {
				t.Fatalf("rendered prompt missing %q:\n%s", want, prompt)
			}
		}
	})
}

type checkpointSummaryTestEnv struct {
	store         *Store
	resolver      *checkpointWorkspaceResolverStub
	workspaceRoot string
	workspaceID   string
	now           time.Time
}

func newCheckpointSummaryTestEnv(t *testing.T) checkpointSummaryTestEnv {
	t.Helper()

	baseDir := t.TempDir()
	workspaceRoot := filepath.Join(baseDir, "workspace")
	if err := os.MkdirAll(workspaceRoot, dirPerm); err != nil {
		t.Fatalf("MkdirAll(workspace) error = %v", err)
	}
	identity, err := workspacepkg.EnsureIdentity(testutil.Context(t), workspaceRoot)
	if err != nil {
		t.Fatalf("EnsureIdentity() error = %v", err)
	}
	store := newOpenTestStore(
		t,
		filepath.Join(baseDir, "home", "memory"),
		WithCatalogDatabasePath(filepath.Join(baseDir, "home", "agh.db")),
	)
	if err := store.EnsureDirs(); err != nil {
		t.Fatalf("Store.EnsureDirs() error = %v", err)
	}
	resolved := workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{
		ID:      identity.WorkspaceID,
		RootDir: workspaceRoot,
	}}
	resolved.WorkspaceID = identity.WorkspaceID
	return checkpointSummaryTestEnv{
		store:         store,
		resolver:      &checkpointWorkspaceResolverStub{resolved: resolved},
		workspaceRoot: workspaceRoot,
		workspaceID:   identity.WorkspaceID,
		now:           time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC),
	}
}

func (e checkpointSummaryTestEnv) sessionEndRecord(sessionID string, fact string) memcontract.SessionEndRecord {
	return memcontract.SessionEndRecord{
		WorkspaceID: e.workspaceID,
		SessionID:   sessionID,
		AgentName:   "coder",
		EndedAt:     e.now,
		Snapshot: memcontract.TranscriptSnapshot{Messages: []memcontract.TranscriptMessage{{
			Sequence: 1,
			Role:     "user",
			Content:  fact,
			At:       e.now,
		}}},
	}
}

func (e checkpointSummaryTestEnv) preCompressRequest(
	sessionID string,
	fromSequence int64,
	toSequence int64,
	fact string,
) memcontract.PreCompressRequest {
	message := memcontract.TranscriptMessage{
		Sequence: fromSequence,
		Role:     "user",
		Content:  fact,
		At:       e.now,
	}
	return memcontract.PreCompressRequest{
		WorkspaceID:  e.workspaceID,
		SessionID:    sessionID,
		AgentName:    "coder",
		FromSequence: fromSequence,
		ToSequence:   toSequence,
		Snapshot:     memcontract.TranscriptSnapshot{Messages: []memcontract.TranscriptMessage{message}},
	}
}

type checkpointWorkspaceResolverStub struct {
	resolved workspacepkg.ResolvedWorkspace
	err      error
}

func (r *checkpointWorkspaceResolverStub) Resolve(
	_ context.Context,
	_ string,
) (workspacepkg.ResolvedWorkspace, error) {
	if r.err != nil {
		return workspacepkg.ResolvedWorkspace{}, r.err
	}
	return r.resolved, nil
}

type checkpointSummarizerStub struct {
	outputs  []string
	errAt    map[int]error
	requests []CheckpointSummaryRequest
}

func (s *checkpointSummarizerStub) Summarize(
	_ context.Context,
	request CheckpointSummaryRequest,
) (string, error) {
	call := len(s.requests)
	s.requests = append(s.requests, request)
	if err := s.errAt[call]; err != nil {
		return "", err
	}
	if call >= len(s.outputs) {
		return "", errors.New("unexpected checkpoint summarizer call")
	}
	return s.outputs[call], nil
}

func checkpointSummaryFixture(fact string) string {
	return strings.Join([]string{
		checkpointSummaryFirstHeading,
		"None.",
		"## Goal",
		fact,
		"## Constraints & Preferences",
		"None.",
		"## Completed Actions",
		"1. Preserved the fixture fact.",
		"## Active State",
		"Idle.",
		"## Historical In-Progress State",
		"None.",
		"## Blocked",
		"None.",
		"## Key Decisions",
		fact,
		"## Resolved Questions",
		"None.",
		"## Historical Pending User Asks",
		"None.",
		"## Relevant Files",
		"None.",
		"## Historical Remaining Work",
		"None.",
		checkpointSummaryLastHeading,
		fact,
	}, "\n\n")
}

func duplicateCheckpointSummaryHeadingFixture() string {
	return strings.Replace(
		checkpointSummaryFixture("Stable checkpoint fact."),
		checkpointSummaryLastHeading,
		checkpointSummaryLastHeading+"\nNone.\n\n"+checkpointSummaryLastHeading,
		1,
	)
}

func checkpointHeaderCount(headers []memcontract.Header) int {
	count := 0
	for _, header := range headers {
		if header.Filename == CheckpointSummaryFilename {
			count++
		}
	}
	return count
}
