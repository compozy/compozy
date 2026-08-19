package cmdpalette

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"strings"
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
			Row{ID: "third", Title: "Third", Badge: &Badge{Label: "Broken", Tone: "purple"}},
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

func validListViewPayload() ViewPayload {
	return ViewPayload{
		View: ViewContractVersion,
		Sections: []Section{{Rows: []Row{{
			ID: "task-1", Title: "Review task", Badge: &Badge{Label: "Queued", Tone: "info"},
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
