package cmdpalette

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
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
		requireViewValidationPath(t, err, "sections[0].rows[2].badge.tone")
	})

	t.Run("Should reject oversized fields with their wire path", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows[0].Title = strings.Repeat("x", MaxViewTextBytes+1)
		_, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		requireViewValidationPath(t, err, "sections[0].rows[0].title")
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
		want := viewOverflowMessage(MaxViewMountRows, MaxViewMountRows+17)
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
		requireViewValidationPath(t, err, "sections[0].rows[0].actions[0]")

		payload.Sections[0].Rows[0].Actions = []RowAction{{Title: "Delete", Handler: "h1", Destructive: true}}
		_, err = ValidateViewPayload(ViewKindList, payload, nil, nil)
		requireViewValidationPath(t, err, "sections[0].rows[0].actions[0].confirmation")
	})

	t.Run("Should accept host-target copy row actions and reject invalid copy payloads", func(t *testing.T) {
		t.Parallel()
		payload := validListViewPayload()
		payload.Sections[0].Rows[0].Actions = []RowAction{{
			Title: "Copy",
			Action: &Action{
				Kind: ActionKindCopy,
				Args: map[string]any{"content": "clipboard text"},
			},
		}}
		if _, err := ValidateViewPayload(ViewKindList, payload, nil, nil); err != nil {
			t.Fatalf("ValidateViewPayload() error = %v", err)
		}

		payload.Sections[0].Rows[0].Actions = []RowAction{{
			Title:  "Copy",
			Action: &Action{Kind: ActionKindCopy},
		}}
		_, err := ValidateViewPayload(ViewKindList, payload, nil, nil)
		requireViewValidationPath(t, err, "sections[0].rows[0].actions[0].action")
		requireErrorContains(t, err, "requires its target")

		payload.Sections[0].Rows[0].Actions = []RowAction{{
			Title: "Copy",
			Action: &Action{
				Kind: ActionKindCopy,
				URL:  "https://example.com",
				Args: map[string]any{"content": "clipboard text"},
			},
		}}
		_, err = ValidateViewPayload(ViewKindList, payload, nil, nil)
		requireViewValidationPath(t, err, "sections[0].rows[0].actions[0].action")
		requireErrorContains(t, err, "cannot carry")
	})

	t.Run("Should return an honest unknown kind error", func(t *testing.T) {
		t.Parallel()
		_, err := ValidateViewPayload(ViewKind("canvas"), ViewPayload{View: ViewContractVersion}, nil, nil)
		var kindErr *UnknownViewKindError
		if !errors.As(err, &kindErr) || kindErr.Kind != "canvas" {
			t.Fatalf("ValidateViewPayload() error = %#v, want canvas UnknownViewKindError", err)
		}
	})

	t.Run("Should cap form field text at the per-field host limit", func(t *testing.T) {
		t.Parallel()
		payload := ViewPayload{
			View: ViewContractVersion,
			Form: &FormBody{Fields: []FormField{{
				ID: "title", Type: "text", Label: strings.Repeat("x", MaxViewTextBytes+1),
			}}},
		}
		_, err := ValidateViewPayload(ViewKindForm, payload, nil, nil)
		requireViewValidationPath(t, err, "form.fields[0].label")
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

	t.Run("Should reject non-RFC array indices", func(t *testing.T) {
		t.Parallel()
		current := validListViewPayload()
		_, _, _, err := ApplyViewPatch(
			ViewKindList, "vr_1", current,
			ViewPatch{
				ViewID: "ext.notes.recent", From: "vr_1", To: "vr_2",
				Ops: []PatchOp{{
					Op: "replace", Path: "/sections/01/rows/0/title", Value: json.RawMessage(`"Changed"`),
				}},
			}, nil, nil,
		)
		requireErrorContains(t, err, `invalid array index "01"`)
	})

	t.Run("Should preserve deterministic replacement properties", func(t *testing.T) {
		t.Parallel()
		for index := range 100 {
			current := validListViewPayload()
			value := fmt.Sprintf("row-%d", index)
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
		_, err := service.OpenSource(t.Context(), testProfileLens, "workspace-a", "ext.notes.recent")
		requireViewValidationPath(t, err, "source.tool")
		if provider.calls != 0 {
			t.Fatalf("provider calls = %d, want zero before policy validation", provider.calls)
		}

		service.viewProviders[0].Descriptor.Source.ReadOnly = true
		snapshot, err := service.OpenSource(t.Context(), testProfileLens, "workspace-a", "ext.notes.recent")
		if err != nil {
			t.Fatalf("OpenSource() error = %v", err)
		}
		if provider.workspaceID != "workspace-a" || snapshot.Revision == "" ||
			snapshot.StreamEpoch != "vse_test" || snapshot.ProfileLens != testProfileLens {
			t.Fatalf("OpenSource() snapshot/provider = %#v / %#v", snapshot, provider)
		}
		aggregate, err := service.OpenSource(t.Context(), AggregateProfileLens(), "workspace-a", "ext.notes.recent")
		if err != nil {
			t.Fatalf("OpenSource(aggregate) error = %v", err)
		}
		if aggregate.ProfileLens != AggregateProfileLens() || aggregate.Revision == snapshot.Revision {
			t.Fatalf("aggregate snapshot = %#v, want aggregate lens and distinct revision", aggregate)
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
		_, err := service.OpenSource(t.Context(), testProfileLens, "workspace-a", "ext.notes.form")
		requireViewValidationPath(t, err, "form")
	})

	t.Run("Should subscribe after a replay cursor then snapshot, and type stream failures", func(t *testing.T) {
		t.Parallel()
		events := make(chan ViewPatchEvent)
		close(events)
		provider := &viewPatchSubscriberStub{
			viewSourceProviderStub: viewSourceProviderStub{payload: validListViewPayload()},
			events:                 events,
		}
		service := &Service{
			viewStreamEpoch: "vse_test",
			viewProviders: []ViewProviderRegistration{{
				Descriptor: ViewDescriptor{
					ID: "ext.notes.recent", Title: "Recent notes", Kind: ViewKindList,
					Source: &ViewToolSource{Tool: "list_recent", ReadOnly: true},
				},
				Provider: provider,
			}},
		}

		_, _, _, err := service.SubscribeViewPatches(
			t.Context(),
			ViewPatchSubscribeRequest{
				ProfileLens: testProfileLens,
				Workspace:   "workspace-a",
				ViewID:      "ext.notes.recent",
				After:       -1,
			},
		)
		if !errors.Is(err, ErrViewInvalidSequence) {
			t.Fatalf("SubscribeViewPatches(after=-1) error = %v, want ErrViewInvalidSequence", err)
		}

		_, _, _, err = service.SubscribeViewPatches(
			t.Context(),
			ViewPatchSubscribeRequest{
				ProfileLens: testProfileLens,
				Workspace:   "workspace-a",
				ViewID:      "ext.notes.recent",
				After:       4,
			},
		)
		if !errors.Is(err, ErrViewStreamEpochRequired) {
			t.Fatalf("SubscribeViewPatches(missing epoch) error = %v, want ErrViewStreamEpochRequired", err)
		}

		initialSnapshot, _, initialCancel, err := service.SubscribeViewPatches(
			t.Context(),
			ViewPatchSubscribeRequest{
				ProfileLens: testProfileLens,
				Workspace:   "workspace-a",
				ViewID:      "ext.notes.recent",
			},
		)
		if err != nil {
			t.Fatalf("SubscribeViewPatches(initial) error = %v", err)
		}
		initialCancel()
		if request := provider.subscribeRequest(); request.StreamEpoch != initialSnapshot.StreamEpoch {
			t.Fatalf(
				"SubscribeViewPatches(initial) request epoch = %q, want snapshot epoch %q",
				request.StreamEpoch,
				initialSnapshot.StreamEpoch,
			)
		}

		snapshot, stream, cancel, err := service.SubscribeViewPatches(
			t.Context(),
			ViewPatchSubscribeRequest{
				ProfileLens: testProfileLens,
				Workspace:   "workspace-a",
				ViewID:      "ext.notes.recent",
				After:       4,
				StreamEpoch: "vse_prior",
			},
		)
		if err != nil {
			t.Fatalf("SubscribeViewPatches() error = %v", err)
		}
		cancel()
		if snapshot.StreamEpoch != "vse_test" || snapshot.Revision == "" {
			t.Fatalf("SubscribeViewPatches() snapshot = %#v, want current fence", snapshot)
		}
		if _, open := <-stream; open {
			t.Fatal("SubscribeViewPatches() stream stayed open after the fixture closed")
		}
		if got := provider.subscribeThenOpen(); !got {
			t.Fatalf("provider order = %v, want subscribe before open", provider.ops())
		}
		if request := provider.subscribeRequest(); request.After != 4 || request.StreamEpoch != "vse_prior" ||
			request.Workspace != "workspace-a" || request.ViewID != "ext.notes.recent" {
			t.Fatalf("SubscribeViewPatches() request = %#v", request)
		}

		missing := &Service{viewProviders: []ViewProviderRegistration{{
			Descriptor: ViewDescriptor{
				ID: "ext.notes.recent", Title: "Recent notes", Kind: ViewKindList,
				Source: &ViewToolSource{Tool: "list_recent", ReadOnly: true},
			},
			Provider: &viewSourceProviderStub{payload: validListViewPayload()},
		}}}
		_, _, _, err = missing.SubscribeViewPatches(
			t.Context(),
			ViewPatchSubscribeRequest{
				ProfileLens: testProfileLens,
				Workspace:   "workspace-a",
				ViewID:      "ext.notes.recent",
			},
		)
		if !errors.Is(err, ErrViewPatchStreamUnavailable) {
			t.Fatalf(
				"SubscribeViewPatches(no subscriber) error = %v, want ErrViewPatchStreamUnavailable",
				err,
			)
		}

		failing := &viewPatchSubscriberStub{
			viewSourceProviderStub: viewSourceProviderStub{payload: ViewPayload{View: ViewContractVersion}},
			events:                 events,
		}
		broken := &Service{viewProviders: []ViewProviderRegistration{{
			Descriptor: ViewDescriptor{
				ID: "ext.notes.form", Title: "Note form", Kind: ViewKindForm,
				Source: &ViewToolSource{Tool: "note_form", ReadOnly: true},
			},
			Provider: failing,
		}}}
		_, _, _, err = broken.SubscribeViewPatches(
			t.Context(),
			ViewPatchSubscribeRequest{
				ProfileLens: testProfileLens,
				Workspace:   "workspace-a",
				ViewID:      "ext.notes.form",
			},
		)
		if err == nil {
			t.Fatal("SubscribeViewPatches(invalid snapshot) error = nil")
		}
		if failing.cancelCount() != 1 {
			t.Fatalf("cancel after snapshot failure = %d, want 1", failing.cancelCount())
		}
		if got := failing.subscribeThenOpen(); !got {
			t.Fatalf("failed-open order = %v, want subscribe before open", failing.ops())
		}
	})
}

func TestViewProgramSessions(t *testing.T) {
	t.Parallel()

	t.Run("Should emit the correlated programmable-view lifecycle [IT-033]", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		recorder := &recordingEventRecorder{wake: make(chan struct{}, 16)}
		service := testProgramViewService(t, testProgramViewClients("client-a"), program, WithEventRecorder(recorder))
		service.viewAckBudget = 10 * time.Millisecond
		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", Client: "client-a", AttachmentToken: "token-a",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		token := SessionToken{ViewSession: opened.Token.ViewSession, AttachmentToken: "token-a"}
		for seq := int64(1); seq <= viewSessionCircuitMisses; seq++ {
			if err := service.AdmitEvent(t.Context(), token, ViewEvent{
				Handler: "h_action", Args: []any{"row", seq}, Revision: "vr_1", Seq: seq,
			}); err != nil {
				t.Fatalf("AdmitEvent(seq=%d) error = %v", seq, err)
			}
		}
		waitForRecordedEvents(t, recorder, 1+viewSessionCircuitMisses)
		if err := service.CloseSession(t.Context(), token, "palette_dismissed"); err != nil {
			t.Fatalf("CloseSession() error = %v", err)
		}
		waitForRecordedEvents(t, recorder, 2+viewSessionCircuitMisses)

		events := recorder.recorded()
		if len(events) != 5 {
			t.Fatalf("recorded events = %#v, want 5 lifecycle events", events)
		}
		if events[0].Name != EventViewSessionOpened || events[len(events)-1].Name != EventViewSessionClosed {
			t.Fatalf("lifecycle endpoints = %q / %q, want opened / closed", events[0].Name, events[len(events)-1].Name)
		}
		degraded, circuitBroken := 0, 0
		for index, event := range events {
			switch event.Name {
			case EventViewSessionDegraded:
				degraded++
			case EventViewSessionCircuitBroken:
				circuitBroken++
			}
			if event.WorkspaceID != "workspace-a" || event.ViewID != "ext.notes.browser" ||
				event.Extension != "notes" || event.ClientID != "client-a" ||
				event.ViewSessionID != opened.Token.ViewSession {
				t.Fatalf("event[%d] correlation = %#v", index, event)
			}
		}
		if degraded != 2 || circuitBroken != 1 {
			t.Fatalf("lifecycle misses = degraded:%d circuit:%d, want 2 / 1", degraded, circuitBroken)
		}
	})

	t.Run("Should keep frame pushes and admitted keystrokes out of the event matrix [IT-033]", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		recorder := &recordingEventRecorder{}
		service := testProgramViewService(t, testProgramViewClients("client-a"), program, WithEventRecorder(recorder))
		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", Client: "client-a", AttachmentToken: "token-a",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		extensionToken := SessionToken{ViewSession: opened.Token.ViewSession, Extension: "notes"}
		if err := service.PublishFrame(
			t.Context(),
			extensionToken,
			viewProgramFrame(opened.Token.ViewSession, "vr_push", 0, 0),
		); err != nil {
			t.Fatalf("PublishFrame() error = %v", err)
		}
		clientToken := SessionToken{
			ViewSession: opened.Token.ViewSession, AttachmentToken: "token-a",
		}
		if err := service.AdmitEvent(t.Context(), clientToken, ViewEvent{
			Handler: "h_search", Args: []any{"query", 1}, Revision: "vr_push", Seq: 1,
		}); err != nil {
			t.Fatalf("AdmitEvent() error = %v", err)
		}
		if events := recorder.recorded(); len(events) != 1 || events[0].Name != EventViewSessionOpened {
			t.Fatalf("events after push and keystroke = %#v, want opened only", events)
		}
		if err := service.CloseSession(t.Context(), clientToken, "palette_dismissed"); err != nil {
			t.Fatalf("CloseSession() error = %v", err)
		}
	})

	t.Run("Should admit generation-zero pushes only before the first event", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		service := testProgramViewService(t, testProgramViewClients("client-a"), program)
		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
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

	t.Run("Should reject executable OpenURL effect schemes", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		service := testProgramViewService(t, testProgramViewClients("client-a"), program)
		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", AttachmentToken: "token-a", View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		frame := viewProgramFrame(opened.Token.ViewSession, "vr_url", 0, 0)
		frame.Effects = []Effect{{ID: "ef_js", OpenURL: &OpenURLEffect{URL: "javascript:alert(1)"}}}
		if err := service.PublishFrame(
			t.Context(),
			SessionToken{ViewSession: opened.Token.ViewSession, Extension: "notes"},
			frame,
		); !errors.Is(err, ErrUnsafeURL) {
			t.Fatalf("PublishFrame(javascript URL) error = %v, want ErrUnsafeURL", err)
		}
	})

	t.Run("Should bind sessions and fence superseded output and acknowledged effects", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		service := testProgramViewService(t, testProgramViewClients("client-a", "client-b"), program)

		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", AttachmentToken: "token-a", View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		if opened.FirstFrame.ViewSession != opened.Token.ViewSession || opened.Token.StreamToken == "" {
			t.Fatalf("OpenSession() = %#v, want bound session and stream token", opened)
		}
		if program.lastOpen.Client != "client-a" || program.lastOpen.Workspace != "workspace-a" ||
			program.lastOpen.ProfileLens != testProfileLens {
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
		if first.event.Generation != 1 || first.profileLens != testProfileLens || first.workspaceID != "workspace-a" {
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
				Handler:  "h_action",
				Args:     []any{"row", seq},
				Revision: "vr_2",
				Seq:      seq,
			}); err != nil {
				t.Fatalf("AdmitEvent(action seq=%d) error = %v", seq, err)
			}
		}
		busyEvent := ViewEvent{
			Handler:  "h_action",
			Args:     []any{"row", 7},
			Revision: "vr_2",
			Seq:      7,
		}
		if err := service.AdmitEvent(t.Context(), clientToken, busyEvent); !errors.Is(err, ErrViewBusy) {
			t.Fatalf("AdmitEvent(over cap) error = %v, want ErrViewBusy", err)
		}
		if err := service.AdmitEvent(t.Context(), clientToken, busyEvent); !errors.Is(err, ErrViewBusy) {
			t.Fatalf("AdmitEvent(retry busy seq) error = %v, want ErrViewBusy", err)
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

	t.Run("Should reject invalid, non-increasing, and stale view events with typed causes", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		service := testProgramViewService(t, testProgramViewClients("client-a"), program)
		opened, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", Client: "client-a", AttachmentToken: "token-a",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession() error = %v", err)
		}
		token := SessionToken{ViewSession: opened.Token.ViewSession, AttachmentToken: "token-a"}
		if err := service.AdmitEvent(t.Context(), token, ViewEvent{
			Revision: "vr_1",
			Seq:      1,
		}); !errors.Is(err, ErrViewEventInvalid) {
			t.Fatalf("AdmitEvent(missing handler) error = %v, want ErrViewEventInvalid", err)
		}
		firstEvent := ViewEvent{
			Handler:  "h_action",
			Args:     []any{"row"},
			Revision: "vr_1",
			Seq:      1,
		}
		if err := service.AdmitEvent(t.Context(), token, firstEvent); err != nil {
			t.Fatalf("AdmitEvent(seq=1) error = %v", err)
		}
		if err := service.AdmitEvent(t.Context(), token, firstEvent); !errors.Is(err, ErrViewEventSeqNotIncreasing) {
			t.Fatalf("AdmitEvent(non-increasing) error = %v, want ErrViewEventSeqNotIncreasing", err)
		}
		staleEvent := ViewEvent{
			Handler:  "h_action",
			Args:     []any{"row"},
			Revision: "vr_stale",
			Seq:      2,
		}
		if err := service.AdmitEvent(t.Context(), token, staleEvent); !errors.Is(err, ErrViewEventRevisionStale) {
			t.Fatalf("AdmitEvent(stale revision) error = %v, want ErrViewEventRevisionStale", err)
		}
	})

	t.Run("Should isolate clients and invalidate replaced extension generations", func(t *testing.T) {
		t.Parallel()
		program := newViewProgramProviderStub()
		service := testProgramViewService(t, testProgramViewClients("client-a", "client-b"), program)
		first, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", Client: "client-a", AttachmentToken: "token-a",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession(client-a) error = %v", err)
		}
		second, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: testProfileLens,
			Workspace: "workspace-a", Client: "client-b", AttachmentToken: "token-b",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession(client-b) error = %v", err)
		}
		if first.Token.ViewSession == second.Token.ViewSession || first.Token.StreamToken == second.Token.StreamToken {
			t.Fatalf("client sessions share identity: %#v / %#v", first.Token, second.Token)
		}
		profileB := ScopedProfileLens("01ARZ3NDEKTSV4RRFFQ69G5FAV", "profile-b")
		third, err := service.OpenSession(t.Context(), ViewSessionOpenRequest{ProfileLens: profileB,
			Workspace: "workspace-a", Client: "client-a", AttachmentToken: "token-a",
			View: "ext.notes.browser",
		})
		if err != nil {
			t.Fatalf("OpenSession(profile-b) error = %v", err)
		}
		if err := service.AckEffects(t.Context(), SessionToken{
			ViewSession: first.Token.ViewSession, AttachmentToken: "token-b",
		}, nil); !errors.Is(err, ErrViewSessionForbidden) {
			t.Fatalf("foreign client access error = %v, want ErrViewSessionForbidden", err)
		}
		if err := service.CloseClientSessions(t.Context(), testProfileLens, "workspace-a", "client-a"); err != nil {
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
		if _, _, cancel, err := service.SubscribeSessionFrames(t.Context(), third.Token); err != nil {
			t.Fatalf("SubscribeSessionFrames(other profile) error = %v, want surviving session", err)
		} else {
			cancel()
		}
		if err := service.InvalidateInstance(t.Context(), testProfileLens, "workspace-a", "notes", 8); err != nil {
			t.Fatalf("InvalidateInstance() error = %v", err)
		}
		if _, _, _, err := service.SubscribeSessionFrames(
			t.Context(),
			second.Token,
		); !errors.Is(err, ErrViewSessionGone) {
			t.Fatalf("SubscribeSessionFrames(second invalidated) error = %v, want ErrViewSessionGone", err)
		}
		if err := service.CloseClientSessions(t.Context(), profileB, "workspace-a", "client-a"); err != nil {
			t.Fatalf("CloseClientSessions(other profile) error = %v", err)
		}
		if _, _, _, err := service.SubscribeSessionFrames(
			t.Context(),
			third.Token,
		); !errors.Is(
			err,
			ErrViewSessionGone,
		) {
			t.Fatalf("SubscribeSessionFrames(other profile after close) error = %v, want ErrViewSessionGone", err)
		}
		if program.closeCount() != 2 {
			t.Fatalf("view/close calls after profile isolation = %d, want 2", program.closeCount())
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
	profileLens ProfileLens
	workspaceID WorkspaceID
	calls       int
}

type viewPatchSubscriberStub struct {
	viewSourceProviderStub
	mu           sync.Mutex
	recordedOps  []string
	request      ViewPatchSubscribeRequest
	cancels      int
	events       <-chan ViewPatchEvent
	subscribeErr error
}

var (
	_ ViewSourceProvider  = (*viewPatchSubscriberStub)(nil)
	_ ViewPatchSubscriber = (*viewPatchSubscriberStub)(nil)
)

func (s *viewPatchSubscriberStub) OpenSource(
	ctx context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	viewID string,
) (ViewPayload, error) {
	s.mu.Lock()
	s.recordedOps = append(s.recordedOps, "open")
	s.mu.Unlock()
	return s.viewSourceProviderStub.OpenSource(ctx, profileLens, workspaceID, viewID)
}

func (s *viewPatchSubscriberStub) SubscribeViewPatches(
	_ context.Context,
	request ViewPatchSubscribeRequest,
) (<-chan ViewPatchEvent, func(), error) {
	s.mu.Lock()
	s.recordedOps = append(s.recordedOps, "subscribe")
	s.request = request
	err := s.subscribeErr
	events := s.events
	s.mu.Unlock()
	if err != nil {
		return nil, nil, err
	}
	if events == nil {
		closed := make(chan ViewPatchEvent)
		close(closed)
		events = closed
	}
	return events, s.cancel, nil
}

func (s *viewPatchSubscriberStub) cancel() {
	s.mu.Lock()
	s.cancels++
	s.mu.Unlock()
}

func (s *viewPatchSubscriberStub) ops() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.recordedOps...)
}

func (s *viewPatchSubscriberStub) subscribeThenOpen() bool {
	subscribeAt, openAt := -1, -1
	for index, op := range s.ops() {
		if op == "subscribe" && subscribeAt < 0 {
			subscribeAt = index
		}
		if op == "open" && openAt < 0 {
			openAt = index
		}
	}
	return subscribeAt >= 0 && openAt > subscribeAt
}

func (s *viewPatchSubscriberStub) subscribeRequest() ViewPatchSubscribeRequest {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.request
}

func (s *viewPatchSubscriberStub) cancelCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cancels
}

type viewProgramEventCall struct {
	ctx         context.Context
	profileLens ProfileLens
	workspaceID WorkspaceID
	event       ViewEvent
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
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	_ string,
	event ViewEvent,
) (*ViewFrame, error) {
	s.events <- viewProgramEventCall{ctx: ctx, profileLens: profileLens, workspaceID: workspaceID, event: event}
	<-ctx.Done()
	return nil, ctx.Err()
}

func (s *viewProgramProviderStub) CloseProgram(
	_ context.Context,
	_ ProfileLens,
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
	payload.Chrome = &ViewChrome{OnSearch: "h_search"}
	return ViewFrame{
		ViewSession: sessionID, Revision: revision, Generation: generation, InReplyTo: reply,
		Payload: &payload, Handlers: []string{"h_search", "h_action"},
	}
}

func requireViewValidationPath(t *testing.T, err error, path string) {
	t.Helper()
	var validation *ViewValidationError
	if !errors.As(err, &validation) {
		t.Fatalf("error = %#v, want ViewValidationError for %q", err, path)
	}
	if validation.Path != path {
		t.Fatalf("validation path = %q, want %q (error = %v)", validation.Path, path, err)
	}
}

func requireErrorContains(t *testing.T, err error, fragment string) {
	t.Helper()
	if err == nil || !strings.Contains(err.Error(), fragment) {
		t.Fatalf("error = %v, want substring %q", err, fragment)
	}
}

func testProgramViewClients(ids ...ClientID) *testClientDirectory {
	clients := make([]Client, 0, len(ids))
	tokens := make(map[ClientID]string, len(ids))
	for _, id := range ids {
		clients = append(clients, Client{ID: id, WorkspaceID: "workspace-a"})
		tokens[id] = "token-" + strings.TrimPrefix(string(id), "client-")
	}
	return &testClientDirectory{clients: clients, tokens: tokens}
}

func testProgramViewService(
	t *testing.T,
	clients ClientDirectory,
	program ViewProgramProvider,
	options ...Option,
) *Service {
	t.Helper()
	options = append([]Option{
		WithViewProviders([]ViewProviderRegistration{{
			Descriptor: ViewDescriptor{
				ID: "ext.notes.browser", Title: "Notes", Kind: ViewKindList,
				Program: true, Extension: "notes",
			},
			Provider: &viewSourceProviderStub{},
		}}),
		WithViewProgramProvider(program),
	}, options...)
	return testRegistryWithOptions(
		staticTestProvider{commands: []Descriptor{testDescriptor("test.command")}},
		clients,
		nil,
		&testExecutor{},
		options...,
	)
}

func waitForRecordedEvents(t *testing.T, recorder *recordingEventRecorder, count int) {
	t.Helper()
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	for len(recorder.recorded()) < count {
		select {
		case <-recorder.wake:
		case <-timer.C:
			t.Fatalf("recorded events = %#v, want at least %d", recorder.recorded(), count)
		}
	}
}

func (s *viewSourceProviderStub) OpenSource(
	_ context.Context,
	profileLens ProfileLens,
	workspaceID WorkspaceID,
	_ string,
) (ViewPayload, error) {
	s.calls++
	s.profileLens = profileLens
	s.workspaceID = workspaceID
	return s.payload, nil
}

func (s *viewCapabilityReporterStub) RecordCapabilityGap(path string, _ map[string]string) {
	s.paths = append(s.paths, path)
}
