package daemon

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

func TestSectionSelectorQueuesStartupSummariesUntilSessionCreated(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 18, 12, 0, 0, 0, time.UTC)
	recorder := newHarnessLifecycleRecorder(discardLogger(), func() time.Time { return base })
	summaryStore := &recordingHarnessSummaryStore{}
	recorder.SetStore(summaryStore)

	resolver := NewHarnessContextResolver(HarnessRuntimeSignals{
		MemoryPromptSectionEnabled: true,
		SkillsPromptSectionEnabled: true,
	})
	selector := NewSectionSelector(resolver, recorder)
	descriptors := defaultStartupPromptSectionDescriptors(
		promptSectionProviderFunc(
			func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) { return "memory", nil },
		),
		promptSectionProviderFunc(
			func(context.Context, *workspacepkg.ResolvedWorkspace) (string, error) { return "skills", nil },
		),
		nil,
	)

	startup := session.StartupPromptContext{
		SessionID:            "sess-startup",
		AgentName:            "coder",
		SessionType:          session.SessionTypeUser,
		NetworkParticipation: daemonTestLiveParticipation("ws-1", "builders"),
	}
	selected, _, err := selector.Select(startup, descriptors)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}
	if got, want := len(selected), 3; got != want {
		t.Fatalf("len(selected) = %d, want %d", got, want)
	}
	if got := summaryStore.Summaries(); len(got) != 0 {
		t.Fatalf("startup summaries written before session creation = %#v, want queued only", got)
	}

	recorder.OnSessionCreated(context.Background(), &session.Session{ID: startup.SessionID})

	summaries := summaryStore.Summaries()
	if got, want := len(summaries), 2; got != want {
		t.Fatalf("len(flushed summaries) = %d, want %d", got, want)
	}
	if got, want := summaries[0].Type, harnessSummaryContextResolved; got != want {
		t.Fatalf("summaries[0].Type = %q, want %q", got, want)
	}
	if got, want := summaries[1].Type, harnessSummarySectionSelected; got != want {
		t.Fatalf("summaries[1].Type = %q, want %q", got, want)
	}
	if got, want := summaries[0].SessionID, startup.SessionID; got != want {
		t.Fatalf("summaries[0].SessionID = %q, want %q", got, want)
	}
	if got, want := summaries[0].AgentName, startup.AgentName; got != want {
		t.Fatalf("summaries[0].AgentName = %q, want %q", got, want)
	}
	if got, want := summaries[0].Timestamp, base; !got.Equal(want) {
		t.Fatalf("summaries[0].Timestamp = %v, want %v", got, want)
	}
	if got, want := summaries[0].RootSessionID, startup.SessionID; got != want {
		t.Fatalf("summaries[0].RootSessionID = %q, want %q", got, want)
	}
	if !strings.Contains(summaries[0].Summary, "surface=startup") {
		t.Fatalf("context summary = %q, want startup surface", summaries[0].Summary)
	}
	if !strings.Contains(summaries[0].Summary, "sections=memory|skills|network") {
		t.Fatalf("context summary = %q, want selected section list", summaries[0].Summary)
	}
	if !strings.Contains(summaries[1].Summary, "selected=memory|skills|network") {
		t.Fatalf("section summary = %q, want selected section names", summaries[1].Summary)
	}
}

func TestPromptInputCompositeRecordsHarnessAugmenterObservability(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 18, 12, 30, 0, 0, time.UTC)
	recorder := newHarnessLifecycleRecorder(discardLogger(), func() time.Time { return base })
	summaryStore := &recordingHarnessSummaryStore{}
	recorder.SetStore(summaryStore)

	resolver := &staticPromptInputAugmenterResolver{
		resolved: ResolvedHarnessContext{
			Surface: ResolutionSurfaceTurn,
			Session: HarnessSessionContext{
				Type:         session.SessionTypeUser,
				SessionClass: SessionClassInteractive,
			},
			Turn: HarnessTurnContext{
				Origin: TurnOriginUser,
			},
			Policy: ResolvedHarnessPolicy{
				SessionClass:     SessionClassInteractive,
				TurnOrigin:       TurnOriginUser,
				EnableAugmenters: []HarnessAugmenter{"warn", "suffix"},
				DiagnosticLabel:  "interactive.user",
			},
		},
	}

	augmenter, err := newPromptInputCompositeAugmenter(
		discardLogger(),
		resolver,
		recorder,
		promptInputAugmenterDescriptor{
			Name:     "warn",
			Order:    100,
			Budget:   32,
			Critical: true,
			Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
				return "", context.DeadlineExceeded
			},
		},
		promptInputAugmenterDescriptor{
			Name:   "suffix",
			Order:  200,
			Budget: 32,
			Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
				return message + " ok", nil
			},
		},
	)
	if err != nil {
		t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
	}

	_, err = augmenter(context.Background(), newPromptInputTestSession(""), "base")
	if err == nil {
		t.Fatal("Augment() error = nil, want deadline-exceeded abort")
	}
	if !strings.Contains(err.Error(), `prompt augmenter "warn"`) {
		t.Fatalf("Augment() error = %v, want wrapped augmenter name", err)
	}

	summaries := summaryStore.Summaries()
	gotTypes := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		gotTypes = append(gotTypes, summary.Type)
	}
	wantTypes := []string{
		harnessSummaryContextResolved,
		harnessSummaryAugmenterFailed,
	}
	if !slices.Equal(gotTypes, wantTypes) {
		t.Fatalf("summary types = %#v, want %#v", gotTypes, wantTypes)
	}
	if !strings.Contains(summaries[0].Summary, "surface=turn") {
		t.Fatalf("context summary = %q, want turn surface", summaries[0].Summary)
	}
	if !strings.Contains(summaries[1].Summary, "disposition=abort") {
		t.Fatalf("failure summary = %q, want abort disposition", summaries[1].Summary)
	}
	if !strings.Contains(summaries[1].Summary, `augmenter=warn`) {
		t.Fatalf("failure summary = %q, want augmenter name", summaries[1].Summary)
	}
}

func TestPromptInputCompositeRecordsWarningAndContinuationSummaries(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 4, 18, 12, 45, 0, 0, time.UTC)
	recorder := newHarnessLifecycleRecorder(discardLogger(), func() time.Time { return base })
	summaryStore := &recordingHarnessSummaryStore{}
	recorder.SetStore(summaryStore)

	resolver := &staticPromptInputAugmenterResolver{
		resolved: ResolvedHarnessContext{
			Surface: ResolutionSurfaceTurn,
			Session: HarnessSessionContext{
				Type:         session.SessionTypeUser,
				SessionClass: SessionClassInteractive,
			},
			Turn: HarnessTurnContext{
				Origin: TurnOriginUser,
			},
			Policy: ResolvedHarnessPolicy{
				SessionClass:     SessionClassInteractive,
				TurnOrigin:       TurnOriginUser,
				EnableAugmenters: []HarnessAugmenter{"warn", "suffix"},
				DiagnosticLabel:  "interactive.user",
			},
		},
	}

	augmenter, err := newPromptInputCompositeAugmenter(
		discardLogger(),
		resolver,
		recorder,
		promptInputAugmenterDescriptor{
			Name:   "warn",
			Order:  100,
			Budget: 32,
			Augmenter: func(_ context.Context, _ *session.Session, _ string) (string, error) {
				return "", errors.New("temporary failure")
			},
		},
		promptInputAugmenterDescriptor{
			Name:   "suffix",
			Order:  200,
			Budget: 32,
			Augmenter: func(_ context.Context, _ *session.Session, message string) (string, error) {
				return message + " ok", nil
			},
		},
	)
	if err != nil {
		t.Fatalf("newPromptInputCompositeAugmenter() error = %v", err)
	}

	got, err := augmenter(context.Background(), newPromptInputTestSession(""), "base")
	if err != nil {
		t.Fatalf("Augment() error = %v, want warning-only continuation", err)
	}
	if got != "base ok" {
		t.Fatalf("Augment() = %q, want suffix continuation", got)
	}

	summaries := summaryStore.Summaries()
	gotTypes := make([]string, 0, len(summaries))
	for _, summary := range summaries {
		gotTypes = append(gotTypes, summary.Type)
	}
	wantTypes := []string{
		harnessSummaryContextResolved,
		harnessSummaryAugmenterFailed,
		harnessSummaryAugmenterApplied,
	}
	if !slices.Equal(gotTypes, wantTypes) {
		t.Fatalf("summary types = %#v, want %#v", gotTypes, wantTypes)
	}
	if !strings.Contains(summaries[1].Summary, "disposition=warn_continue") {
		t.Fatalf("warning summary = %q, want warn-continue disposition", summaries[1].Summary)
	}
	if !strings.Contains(summaries[2].Summary, "outcome=applied") {
		t.Fatalf("applied summary = %q, want applied outcome", summaries[2].Summary)
	}
}

type recordingHarnessSummaryStore struct {
	mu        sync.Mutex
	summaries []store.EventSummary
}

func (s *recordingHarnessSummaryStore) WriteEventSummary(_ context.Context, summary store.EventSummary) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.summaries = append(s.summaries, summary)
	return nil
}

func (s *recordingHarnessSummaryStore) ListEventSummaries(
	_ context.Context,
	query store.EventSummaryQuery,
) ([]store.EventSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]store.EventSummary, 0, len(s.summaries))
	for _, summary := range s.summaries {
		if query.SessionID != "" && summary.SessionID != query.SessionID {
			continue
		}
		if query.Type != "" && summary.Type != query.Type {
			continue
		}
		filtered = append(filtered, summary)
	}
	if query.Limit > 0 && len(filtered) > query.Limit {
		filtered = filtered[len(filtered)-query.Limit:]
	}
	return append([]store.EventSummary(nil), filtered...), nil
}

func (s *recordingHarnessSummaryStore) Summaries() []store.EventSummary {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]store.EventSummary(nil), s.summaries...)
}
