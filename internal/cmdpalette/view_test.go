package cmdpalette

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
	"sync"
	"testing"
)

func TestViewPayloadValidation(t *testing.T) {
	t.Parallel()

	t.Run("Should round trip a valid list payload", func(t *testing.T) {
		t.Parallel()
		count := 2
		payload := validListViewPayload()
		payload.Chips = []Chip{{ID: "open", Label: "Open", Count: &count}}
		payload.Empty = &EmptyState{Title: "No tasks", Hint: "Clear the filter"}

		validated, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err != nil {
			t.Fatalf("ValidateViewPayload() error = %v", err)
		}
		wire, err := json.Marshal(validated)
		if err != nil {
			t.Fatalf("json.Marshal() error = %v", err)
		}
		var roundTrip ViewPayload
		if err := json.Unmarshal(wire, &roundTrip); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if roundTrip.View != ViewContractVersion || len(roundTrip.Sections) != 1 || len(roundTrip.Chips) != 1 {
			t.Fatalf("round trip = %#v, want list payload intact", roundTrip)
		}
		if roundTrip.Empty == nil || roundTrip.Empty.Title != "No tasks" {
			t.Fatalf("round trip empty = %#v, want declared empty state", roundTrip.Empty)
		}
	})

	t.Run("Should truncate oversized detail content with the wire marker", func(t *testing.T) {
		t.Parallel()
		payload := ViewPayload{
			View:   ViewContractVersion,
			Detail: &DetailBody{Markdown: strings.Repeat("é", MaxDetailMarkdown)},
		}
		validated, err := ValidateViewPayload(ViewKindDetail, payload, nil, nil)
		if err != nil {
			t.Fatalf("ValidateViewPayload() error = %v", err)
		}
		if len(validated.Detail.Markdown) > MaxDetailMarkdown {
			t.Fatalf("markdown bytes = %d, want <= %d", len(validated.Detail.Markdown), MaxDetailMarkdown)
		}
		if !strings.HasSuffix(validated.Detail.Markdown, DetailTruncationMark) {
			t.Fatalf(
				"markdown suffix = %q, want truncation marker",
				validated.Detail.Markdown[len(validated.Detail.Markdown)-40:],
			)
		}
	})

	t.Run("Should reject malformed fields with their wire path", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows = append(payload.Sections[0].Rows,
			Row{ID: "second", Title: "Second"},
			Row{ID: "third", Title: "Third", Badge: &ViewBadge{Label: "Broken", Tone: "purple"}},
		)
		_, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "sections[0].rows[2].badge.tone") {
			t.Fatalf("ValidateViewPayload() error = %v, want badge tone path", err)
		}
	})

	t.Run("Should reject oversized fields with their wire path", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows[0].Title = strings.Repeat("x", MaxViewTextBytes+1)
		_, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "sections[0].rows[0].title") {
			t.Fatalf("ValidateViewPayload() error = %v, want row title path", err)
		}
	})

	t.Run("Should cap mounted rows and report the exact overflow", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows = make([]Row, MaxViewMountRows+17)
		for index := range payload.Sections[0].Rows {
			payload.Sections[0].Rows[index] = Row{ID: fmt.Sprintf("row-%d", index), Title: fmt.Sprintf("Row %d", index)}
		}
		validated, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err != nil {
			t.Fatalf("ValidateViewPayload() error = %v", err)
		}
		if got := len(validated.Sections[0].Rows); got != MaxViewMountRows {
			t.Fatalf("mounted rows = %d, want %d", got, MaxViewMountRows)
		}
		want := ViewOverflowMessage(MaxViewMountRows, MaxViewMountRows+17)
		if validated.Empty == nil || validated.Empty.Hint != want {
			t.Fatalf("overflow hint = %#v, want %q", validated.Empty, want)
		}
		if validated.Chrome == nil || validated.Chrome.Pagination == nil || !validated.Chrome.Pagination.HasMore {
			t.Fatalf("pagination = %#v, want has_more", validated.Chrome)
		}
	})

	t.Run("Should validate the row action union and destructive confirmation", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows[0].Actions = []RowAction{{Title: "Broken", Handler: "h1", SubmitForm: true}}
		_, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "exactly one") {
			t.Fatalf("ValidateViewPayload() error = %v, want union error", err)
		}

		payload.Sections[0].Rows[0].Actions = []RowAction{{Title: "Delete", Handler: "h1", Destructive: true}}
		_, err = ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err == nil || !strings.Contains(err.Error(), "confirmation") {
			t.Fatalf("ValidateViewPayload() error = %v, want confirmation error", err)
		}
	})

	t.Run("Should return an honest unknown kind error", func(t *testing.T) {
		t.Parallel()
		_, err := ValidateViewPayload(ViewKind("canvas"), ViewPayload{View: ViewContractVersion}, nil, nil)
		var kindErr *UnknownViewKindError
		if !errors.As(err, &kindErr) || kindErr.Kind != "canvas" {
			t.Fatalf("ValidateViewPayload() error = %#v, want canvas UnknownViewKindError", err)
		}
	})
}

func TestViewCapabilityFallbacks(t *testing.T) {
	t.Parallel()

	t.Run("Should drop an unsupported row and record the capability gap", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows[0].Requires = map[string]string{"adaptiveGrid": "2"}
		payload.Sections[0].Rows[0].Fallback = "drop"
		reporter := &viewCapabilityReporterStub{}

		validated, err := ValidateViewPayload(
			ViewKindList, payload, map[string]string{"adaptiveGrid": "1"}, reporter,
		)
		if err != nil {
			t.Fatalf("ValidateViewPayload() error = %v", err)
		}
		if len(validated.Sections[0].Rows) != 0 {
			t.Fatalf("rows = %#v, want unsupported row dropped", validated.Sections[0].Rows)
		}
		if len(reporter.paths) != 1 || reporter.paths[0] != "sections[0].rows[0]" {
			t.Fatalf("capability paths = %#v, want row path", reporter.paths)
		}
	})

	t.Run("Should replace an unsupported row with its declared fallback", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows[0].Requires = map[string]string{"adaptiveGrid": "2"}
		payload.Sections[0].Rows[0].Fallback = `{"id":"fallback","title":"Basic row"}`

		validated, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		if err != nil {
			t.Fatalf("ValidateViewPayload() error = %v", err)
		}
		if got := validated.Sections[0].Rows[0].ID; got != "fallback" {
			t.Fatalf("fallback row id = %q, want fallback", got)
		}
	})
}

func TestViewPatchApplication(t *testing.T) {
	t.Parallel()

	t.Run("Should request resync without applying a revision gap", func(t *testing.T) {
		t.Parallel()
		current := validListViewPayload()
		patched, revision, resync, err := ApplyViewPatch(
			ViewKindList, "vr_2", current,
			ViewPatch{ViewID: "ext.notes.recent", From: "vr_1", To: "vr_3"}, nil, nil,
		)
		var mismatch *ViewRevisionMismatchError
		if !errors.As(err, &mismatch) || !resync {
			t.Fatalf("ApplyViewPatch() = revision %q resync %t error %#v, want fenced resync", revision, resync, err)
		}
		if revision != "vr_2" || patched.Sections[0].Rows[0].Title != current.Sections[0].Rows[0].Title {
			t.Fatalf("gap changed current payload: revision %q payload %#v", revision, patched)
		}
	})

	t.Run("Should apply in-order replacements and advance revision", func(t *testing.T) {
		t.Parallel()
		current := validListViewPayload()
		patched, revision, resync, err := ApplyViewPatch(
			ViewKindList, "vr_1", current,
			ViewPatch{
				ViewID: "ext.notes.recent", From: "vr_1", To: "vr_2",
				Ops: []PatchOp{{Op: "replace", Path: "/sections/0/rows/0/title", Value: json.RawMessage(`"Changed"`)}},
			}, nil, nil,
		)
		if err != nil {
			t.Fatalf("ApplyViewPatch() error = %v", err)
		}
		if resync || revision != "vr_2" || patched.Sections[0].Rows[0].Title != "Changed" {
			t.Fatalf("ApplyViewPatch() = revision %q resync %t payload %#v", revision, resync, patched)
		}
	})

	t.Run("Should preserve deterministic replacement properties", func(t *testing.T) {
		t.Parallel()
		for index := range 100 {
			current := validListViewPayload()
			value := fmt.Sprintf("row-%d-%d", index, rand.IntN(100_000))
			patched, revision, resync, err := ApplyViewPatch(
				ViewKindList, "vr_a", current,
				ViewPatch{
					ViewID: "ext.notes.recent", From: "vr_a", To: "vr_b",
					Ops: []PatchOp{{
						Op: "replace", Path: "/sections/0/rows/0/title", Value: mustViewJSON(t, value),
					}},
				}, nil, nil,
			)
			if err != nil || resync || revision != "vr_b" || patched.Sections[0].Rows[0].Title != value {
				t.Fatalf(
					"iteration %d: payload %#v revision %q resync %t error %v",
					index,
					patched,
					revision,
					resync,
					err,
				)
			}
		}
	})
}

func TestViewService(t *testing.T) {
	t.Parallel()

	t.Run("Should enforce read-only sources and workspace scope before serving", func(t *testing.T) {
		t.Parallel()
		provider := &viewSourceProviderStub{payload: validListViewPayload()}
		service := &Service{
			viewStreamEpoch: "vse_test",
			viewProviders: []ViewProviderRegistration{{
				Descriptor: ViewDescriptor{
					ID: "ext.notes.recent", Title: "Recent notes", Kind: ViewKindList,
					Source: &ViewToolSource{Tool: "list_recent", ReadOnly: false},
				},
				Provider: provider,
			}},
		}
		_, err := service.OpenSource(t.Context(), "workspace-a", "ext.notes.recent")
		if err == nil || !strings.Contains(err.Error(), "read-only") {
			t.Fatalf("OpenSource() error = %v, want read-only source failure", err)
		}
		if provider.calls != 0 {
			t.Fatalf("provider calls = %d, want zero before policy validation", provider.calls)
		}

		service.viewProviders[0].Descriptor.Source.ReadOnly = true
		snapshot, err := service.OpenSource(t.Context(), "workspace-a", "ext.notes.recent")
		if err != nil {
			t.Fatalf("OpenSource() error = %v", err)
		}
		if provider.workspaceID != "workspace-a" || snapshot.Revision == "" ||
			snapshot.StreamEpoch != "vse_test" {
			t.Fatalf("OpenSource() snapshot/provider = %#v / %#v", snapshot, provider)
		}
	})

	t.Run("Should reject an invalid provider payload before serving it", func(t *testing.T) {
		t.Parallel()
		provider := &viewSourceProviderStub{payload: ViewPayload{View: ViewContractVersion}}
		service := &Service{viewProviders: []ViewProviderRegistration{{
			Descriptor: ViewDescriptor{
				ID: "ext.notes.form", Title: "Note form", Kind: ViewKindForm,
				Source: &ViewToolSource{Tool: "note_form", ReadOnly: true},
			},
			Provider: provider,
		}}}
		_, err := service.OpenSource(t.Context(), "workspace-a", "ext.notes.form")
		if err == nil || !strings.Contains(err.Error(), "detail") && !strings.Contains(err.Error(), "form") {
			t.Fatalf("OpenSource() error = %v, want validated form path", err)
		}
	})
}

func TestViewProgramSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should admit generation-zero pushes only before the first event", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{testDescriptor("test.command")}},
			&testClientDirectory{
				clients: []Client{{ID: "client-a", WorkspaceID: "workspace-a"}},
				tokens:  map[ClientID]string{"client-a": "token-a"},
			},
			nil,
			&testExecutor{},
			WithViewProviders([]ViewProviderRegistration{{
				Descriptor: ViewDescriptor{
					ID: "ext.notes.browser", Title: "Notes", Kind: ViewKindList,
					Program: true, Extension: "notes",
				},
				Provider: &viewSourceProviderStub{},
			}}),
			WithViewProgramProvider(program),
		)
		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{
			Workspace: "workspace-a", AttachmentToken: "token-a", View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		extensionToken := SessionToken{ViewSession: opened.Token.ViewSession, Extension: "notes"}
		beforeEvent := viewProgramFrame(opened.Token.ViewSession, "vr_push", 0, 0)
		if err := service.PublishFrame(t.Context(), extensionToken, beforeEvent); err != nil {
			t.Fatalf("PublishFrame(generation zero before event) error = %v", err)
		}
		clientToken := SessionToken{
			ViewSession: opened.Token.ViewSession, AttachmentToken: "token-a",
		}
		if err := service.AdmitEvent(t.Context(), clientToken, ViewEvent{
			Handler: "h_search", Args: []any{"query", 1}, Revision: "vr_push", Seq: 1,
		}); err != nil {
			t.Fatalf("AdmitEvent() error = %v", err)
		}
		<-program.events
		afterEvent := viewProgramFrame(opened.Token.ViewSession, "vr_late_push", 0, 0)
		if err := service.PublishFrame(t.Context(), extensionToken, afterEvent); !errors.Is(err, ErrViewFrameStale) {
			t.Fatalf("PublishFrame(generation zero after event) error = %v, want ErrViewFrameStale", err)
		}
	})

	t.Run("Should bind sessions and fence superseded output and acknowledged effects", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		clients := &testClientDirectory{
			clients: []Client{
				{ID: "client-a", WorkspaceID: "workspace-a"},
				{ID: "client-b", WorkspaceID: "workspace-a"},
			},
			tokens: map[ClientID]string{"client-a": "token-a", "client-b": "token-b"},
		}
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{testDescriptor("test.command")}},
			clients,
			nil,
			&testExecutor{},
			WithViewProviders([]ViewProviderRegistration{{
				Descriptor: ViewDescriptor{
					ID: "ext.notes.browser", Title: "Notes", Kind: ViewKindList,
					Program: true, Extension: "notes",
				},
				Provider: &viewSourceProviderStub{},
			}}),
			WithViewProgramProvider(program),
		)

		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{
			Workspace: "workspace-a", AttachmentToken: "token-a", View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		if opened.FirstFrame.ViewSession != opened.Token.ViewSession || opened.Token.StreamToken == "" {
			t.Fatalf("OpenSession() = %#v, want bound session and stream token", opened)
		}
		if program.lastOpen.Client != "client-a" || program.lastOpen.Workspace != "workspace-a" {
			t.Fatalf("view/open request = %#v, want token-resolved client", program.lastOpen)
		}

		foreign := SessionToken{ViewSession: opened.Token.ViewSession, Extension: "other"}
		err = service.PublishFrame(t.Context(), foreign, viewProgramFrame(opened.Token.ViewSession, "vr_2", 1, 0))
		if !errors.Is(err, ErrViewSessionForbidden) {
			t.Fatalf("PublishFrame(foreign) error = %v, want ErrViewSessionForbidden", err)
		}

		clientToken := SessionToken{
			ViewSession: opened.Token.ViewSession, AttachmentToken: "token-a",
		}
		if err := service.AdmitEvent(t.Context(), clientToken, ViewEvent{
			Handler: "h_search", Args: []any{"a", 1}, Revision: "vr_1", Seq: 1,
		}); err != nil {
			t.Fatalf("AdmitEvent(seq=1) error = %v", err)
		}
		first := <-program.events
		if first.event.Generation != 1 {
			t.Fatalf("first generation = %d, want 1", first.event.Generation)
		}
		if err := service.AdmitEvent(t.Context(), clientToken, ViewEvent{
			Handler: "h_search", Args: []any{"ab", 2}, Revision: "vr_1", Seq: 2,
		}); err != nil {
			t.Fatalf("AdmitEvent(seq=2) error = %v", err)
		}
		second := <-program.events
		select {
		case <-first.ctx.Done():
		default:
			t.Fatal("superseded event context was not canceled")
		}
		if second.event.Generation != 2 {
			t.Fatalf("second generation = %d, want 2", second.event.Generation)
		}

		extensionToken := SessionToken{ViewSession: opened.Token.ViewSession, Extension: "notes"}
		stale := viewProgramFrame(opened.Token.ViewSession, "vr_stale", 1, 0)
		if err := service.PublishFrame(t.Context(), extensionToken, stale); !errors.Is(err, ErrViewFrameStale) {
			t.Fatalf("PublishFrame(stale push) error = %v, want ErrViewFrameStale", err)
		}
		fresh := viewProgramFrame(opened.Token.ViewSession, "vr_2", 2, 2)
		fresh.Effects = []Effect{{ID: "ef_1", Toast: &ToastEffect{Tone: "success", Message: "Saved"}}}
		if err := service.PublishFrame(t.Context(), extensionToken, fresh); err != nil {
			t.Fatalf("PublishFrame(fresh) error = %v", err)
		}
		if err := service.AckEffects(t.Context(), clientToken, []string{"ef_1"}); err != nil {
			t.Fatalf("AckEffects() error = %v", err)
		}
		replay, _, cancel, err := service.SubscribeSessionFrames(t.Context(), SessionToken{
			ViewSession: opened.Token.ViewSession, StreamToken: opened.Token.StreamToken,
		})
		if err != nil {
			t.Fatalf("SubscribeSessionFrames() error = %v", err)
		}
		cancel()
		if len(replay.Effects) != 0 {
			t.Fatalf("replay effects = %#v, want acknowledged effect fenced", replay.Effects)
		}

		for seq := int64(3); seq <= 6; seq++ {
			if err := service.AdmitEvent(t.Context(), clientToken, ViewEvent{
				Handler: "h_search", Revision: "vr_2", Seq: seq,
			}); err != nil {
				t.Fatalf("AdmitEvent(action seq=%d) error = %v", seq, err)
			}
		}
		if err := service.AdmitEvent(t.Context(), clientToken, ViewEvent{
			Handler: "h_search", Revision: "vr_2", Seq: 7,
		}); !errors.Is(err, ErrViewBusy) {
			t.Fatalf("AdmitEvent(over cap) error = %v, want ErrViewBusy", err)
		}

		if err := service.CloseSession(t.Context(), clientToken, "palette_dismissed"); err != nil {
			t.Fatalf("CloseSession() error = %v", err)
		}
		if err := service.CloseSession(t.Context(), clientToken, "again"); err != nil {
			t.Fatalf("CloseSession(idempotent) error = %v", err)
		}
		if err := service.PublishFrame(t.Context(), extensionToken, fresh); !errors.Is(err, ErrViewSessionGone) {
			t.Fatalf("PublishFrame(after close) error = %v, want ErrViewSessionGone", err)
		}
		if program.closeCount() != 1 {
			t.Fatalf("view/close calls = %d, want 1", program.closeCount())
		}
	})

	t.Run("Should isolate clients and invalidate replaced extension generations", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		clients := &testClientDirectory{
			clients: []Client{
				{ID: "client-a", WorkspaceID: "workspace-a"},
				{ID: "client-b", WorkspaceID: "workspace-a"},
			},
			tokens: map[ClientID]string{"client-a": "token-a", "client-b": "token-b"},
		}
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{testDescriptor("test.command")}},
			clients,
			nil,
			&testExecutor{},
			WithViewProviders([]ViewProviderRegistration{{
				Descriptor: ViewDescriptor{
					ID: "ext.notes.browser", Title: "Notes", Kind: ViewKindList,
					Program: true, Extension: "notes",
				},
				Provider: &viewSourceProviderStub{},
			}}),
			WithViewProgramProvider(program),
		)
		first, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{
			Workspace: "workspace-a", Client: "client-a", AttachmentToken: "token-a",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession(client-a) error = %v", err)
		}
		second, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{
			Workspace: "workspace-a", Client: "client-b", AttachmentToken: "token-b",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession(client-b) error = %v", err)
		}
		if first.Token.ViewSession == second.Token.ViewSession || first.Token.StreamToken == second.Token.StreamToken {
			t.Fatalf("client sessions share identity: %#v / %#v", first.Token, second.Token)
		}
		if err := service.AckEffects(t.Context(), SessionToken{
			ViewSession: first.Token.ViewSession, AttachmentToken: "token-b",
		}, nil); !errors.Is(err, ErrViewSessionForbidden) {
			t.Fatalf("foreign client access error = %v, want ErrViewSessionForbidden", err)
		}
		if err := service.CloseClientSessions(t.Context(), "workspace-a", "client-a"); err != nil {
			t.Fatalf("CloseClientSessions(client-a) error = %v", err)
		}
		if _, _, _, err := service.SubscribeSessionFrames(
			t.Context(),
			first.Token,
		); !errors.Is(err, ErrViewSessionGone) {
			t.Fatalf("SubscribeSessionFrames(detached client) error = %v, want ErrViewSessionGone", err)
		}
		if _, _, cancel, err := service.SubscribeSessionFrames(
			t.Context(),
			second.Token,
		); err != nil {
			t.Fatalf("SubscribeSessionFrames(client-b) error = %v", err)
		} else {
			cancel()
		}
		if program.closeCount() != 1 {
			t.Fatalf("view/close calls after client detach = %d, want 1", program.closeCount())
		}
		if err := service.InvalidateInstance(t.Context(), "workspace-a", "notes", 8); err != nil {
			t.Fatalf("InvalidateInstance() error = %v", err)
		}
		if _, _, _, err := service.SubscribeSessionFrames(
			t.Context(),
			second.Token,
		); !errors.Is(err, ErrViewSessionGone) {
			t.Fatalf("SubscribeSessionFrames(second invalidated) error = %v, want ErrViewSessionGone", err)
		}
	})
}

func validListViewPayload() ViewPayload {
	return ViewPayload{
		View: ViewContractVersion,
		Sections: []Section{{Rows: []Row{{
			ID: "task-1", Title: "Review task", Badge: &ViewBadge{Label: "Queued", Tone: "info"},
		}}}},
	}
}

func mustViewJSON(t *testing.T, value any) json.RawMessage {
	t.Helper()
	wire, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	return wire
}

type viewCapabilityReporterStub struct {
	paths []string
}

type viewSourceProviderStub struct {
	payload     ViewPayload
	workspaceID WorkspaceID
	calls       int
}

type viewProgramEventCall struct {
	ctx   context.Context
	event ViewEvent
}

type viewProgramProviderStub struct {
	mu         sync.Mutex
	lastOpen   ViewOpenRequest
	closes     []ViewCloseRequest
	events     chan viewProgramEventCall
	generation uint64
}

func newViewProgramProviderStub() *viewProgramProviderStub {
	return &viewProgramProviderStub{events: make(chan viewProgramEventCall, 16), generation: 7}
}

func (s *viewProgramProviderStub) OpenProgram(
	_ context.Context,
	_ string,
	request ViewOpenRequest,
) (ViewFrame, uint64, error) {
	s.mu.Lock()
	s.lastOpen = request
	s.mu.Unlock()
	return viewProgramFrame(request.ViewSession, "vr_1", 0, 0), s.generation, nil
}

func (s *viewProgramProviderStub) HandleProgramEvent(
	ctx context.Context,
	_ WorkspaceID,
	_ string,
	event ViewEvent,
) (*ViewFrame, error) {
	s.events <- viewProgramEventCall{ctx: ctx, event: event}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *viewProgramProviderStub) CloseProgram(
	_ context.Context,
	_ WorkspaceID,
	_ string,
	request ViewCloseRequest,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closes = append(s.closes, request)
	return nil
}

func (s *viewProgramProviderStub) closeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.closes)
}

func viewProgramFrame(sessionID, revision string, generation uint64, reply int64) ViewFrame {
	payload := validListViewPayload()
	return ViewFrame{
		ViewSession: sessionID, Revision: revision, Generation: generation, InReplyTo: reply,
		Payload: &payload, Handlers: []string{"h_search"},
	}
}

func (s *viewSourceProviderStub) OpenSource(
	_ context.Context,
	workspaceID WorkspaceID,
	_ string,
) (ViewPayload, error) {
	s.calls++
	s.workspaceID = workspaceID
	return s.payload, nil
}

func (s *viewCapabilityReporterStub) RecordCapabilityGap(path string, _ map[string]string) {
	s.paths = append(s.paths, path)
}
