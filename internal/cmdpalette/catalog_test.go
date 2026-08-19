package cmdpalette

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// Suite: daemon command-palette catalog.
// Invariant: structural catalog truth is deterministic, complete, unique, and honest about degraded sources.
func TestRegistryCatalog(t *testing.T) {
	t.Parallel()

	t.Run("Should assemble core and extension commands with stable structural metadata [UT-001]", func(t *testing.T) {
		t.Parallel()
		core := testDescriptor("core.test")
		extension := testDescriptor("ext.notes.capture")
		extension.Source = Source{Kind: SourceKindExtension, Extension: "notes"}
		bindings := &testBindings{
			bindings: map[CommandID][]string{core.ID: {"meta+KeyK"}},
			aliases:  map[CommandID]string{extension.ID: "capture"},
		}
		service, err := NewRegistry(
			[]ProviderRegistration{
				{Source: core.Source, Provider: staticTestProvider{commands: []Descriptor{core}}},
				{Source: extension.Source, Provider: staticTestProvider{commands: []Descriptor{extension}}},
			},
			nil,
			bindings,
			&testExecutor{},
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		first, err := service.Catalog(t.Context(), "ws-1", "")
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		second, err := service.Catalog(t.Context(), "ws-1", "")
		if err != nil {
			t.Fatalf("Catalog() second error = %v", err)
		}
		if len(first.Commands) != 2 || len(first.Sources) != 2 {
			t.Fatalf("catalog = %#v, want two commands and two sources", first)
		}
		if first.Revision != second.Revision || !strings.HasPrefix(first.Revision, "cr_") {
			t.Fatalf("revisions = %q, %q, want stable cr_ digest", first.Revision, second.Revision)
		}
		if first.Commands[0].Bindings[0] != "meta+KeyK" || first.Commands[1].Alias == nil ||
			*first.Commands[1].Alias != "capture" {
			t.Fatalf("resolved commands = %#v", first.Commands)
		}
	})

	t.Run(
		"Should change only structural revision inputs and expose context revision separately [UT-003]",
		func(t *testing.T) {
			t.Parallel()
			descriptor := testDescriptor("window.close")
			descriptor.Action = Action{Kind: ActionKindClientOp, Op: "window.close"}
			descriptor.When = []Predicate{{
				Key: ContextWindowFocused, Value: true, Reason: "needs a focused window",
			}}
			provider := &dynamicTestProvider{commands: []Descriptor{descriptor}}
			directory := &testClientDirectory{
				clients: []Client{{ID: "client-a"}, {ID: "client-b"}},
				contexts: map[ClientID]ContextSnapshot{
					"client-a": {Revision: "ctx-a", Values: map[ContextKey]any{ContextWindowFocused: true}},
					"client-b": {Revision: "ctx-b", Values: map[ContextKey]any{ContextWindowFocused: false}},
				},
			}
			bindings := &testBindings{bindings: map[CommandID][]string{}, aliases: map[CommandID]string{}}
			service := testRegistry(provider, directory, bindings, &testExecutor{})
			clientA, err := service.Catalog(t.Context(), "ws-1", "client-a")
			if err != nil {
				t.Fatalf("Catalog(client-a) error = %v", err)
			}
			clientB, err := service.Catalog(t.Context(), "ws-1", "client-b")
			if err != nil {
				t.Fatalf("Catalog(client-b) error = %v", err)
			}
			if clientA.Revision != clientB.Revision || clientA.ContextRevision == clientB.ContextRevision {
				t.Fatalf("catalog revisions = %#v / %#v", clientA, clientB)
			}
			if !clientA.Commands[0].Available || clientB.Commands[0].Available {
				t.Fatalf("availability = %#v / %#v", clientA.Commands[0], clientB.Commands[0])
			}
			provider.commands[0].Title = "Changed title"
			changed, err := service.Catalog(t.Context(), "ws-1", "client-a")
			if err != nil {
				t.Fatalf("Catalog(changed) error = %v", err)
			}
			if changed.Revision == clientA.Revision {
				t.Fatal("descriptor change did not advance structural revision")
			}
		},
	)

	t.Run("Should reject duplicate IDs at boot and dynamic provider load [UT-004]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("duplicate.command")
		_, err := NewRegistry(
			[]ProviderRegistration{
				{
					Source:   Source{Kind: SourceKindCore},
					Provider: staticTestProvider{commands: []Descriptor{descriptor}},
				},
				{Source: Source{Kind: SourceKindExtension, Extension: "notes"}, Provider: staticTestProvider{
					commands: []Descriptor{func() Descriptor {
						duplicate := descriptor
						duplicate.Source = Source{Kind: SourceKindExtension, Extension: "notes"}
						return duplicate
					}()},
				}},
			}, nil, nil, &testExecutor{},
		)
		if !errors.Is(err, ErrInvalidDescriptor) && !errors.Is(err, ErrDuplicateCommandID) {
			t.Fatalf("NewRegistry() error = %v, want invalid extension namespace or duplicate", err)
		}

		extensionDuplicate := descriptor
		extensionDuplicate.Source = Source{Kind: SourceKindExtension, Extension: "notes"}
		extensionDuplicate.ID = descriptor.ID
		service, err := NewRegistry(
			[]ProviderRegistration{
				{Source: descriptor.Source, Provider: dynamicTestProvider{commands: []Descriptor{descriptor}}},
				{
					Source:   extensionDuplicate.Source,
					Provider: dynamicTestProvider{commands: []Descriptor{extensionDuplicate}},
				},
			}, nil, nil, &testExecutor{},
		)
		if err != nil {
			t.Fatalf("NewRegistry(dynamic) error = %v", err)
		}
		_, err = service.Catalog(t.Context(), "ws-1", "")
		if !errors.Is(err, ErrInvalidDescriptor) && !errors.Is(err, ErrDuplicateCommandID) {
			t.Fatalf("Catalog() error = %v, want invalid extension namespace or duplicate", err)
		}

		aggregate := Source{Kind: SourceKindExtension}
		_, err = NewRegistry(
			[]ProviderRegistration{
				{Source: aggregate, Provider: &contributionTestProvider{}},
				{Source: aggregate, Provider: &contributionTestProvider{}},
			},
			nil, nil, &testExecutor{},
		)
		if err == nil || !strings.Contains(err.Error(), `duplicate provider source "extension"`) {
			t.Fatalf("NewRegistry(duplicate aggregate) error = %v, want duplicate provider source", err)
		}
	})

	t.Run("Should retain healthy sources when another provider is degraded [UT-005]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("core.healthy")
		service, err := NewRegistry(
			[]ProviderRegistration{
				{Source: descriptor.Source, Provider: dynamicTestProvider{commands: []Descriptor{descriptor}}},
				{
					Source:   Source{Kind: SourceKindExtension, Extension: "broken"},
					Provider: dynamicTestProvider{err: errors.New("crash loop")},
				},
			}, nil, nil, &testExecutor{},
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		catalog, err := service.Catalog(t.Context(), "ws-1", "")
		if err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if len(catalog.Commands) != 1 || len(catalog.Sources) != 2 ||
			catalog.Sources[1].Status != SourceDegraded || catalog.Sources[1].Reason != "crash loop" {
			t.Fatalf("catalog = %#v, want healthy command and degraded diagnostic", catalog)
		}
	})

	t.Run(
		"Should keep unhealthy extension members unavailable under a new revision [UT-057,UT-060]",
		func(t *testing.T) {
			t.Parallel()
			descriptor := testDescriptor("ext.notes.capture")
			descriptor.Source = Source{Kind: SourceKindExtension, Extension: "notes"}
			provider := &contributionTestProvider{contribution: Contribution{
				Commands: []Descriptor{descriptor},
				Sources:  []SourceStatus{{Source: "ext.notes", Status: SourceHealthy}},
				Defaults: []ExtensionDefaultShortcut{{
					CommandID: descriptor.ID, Chord: "alt+shift+KeyN", Source: "ext.notes", Active: true,
				}},
			}}
			service, err := NewRegistry([]ProviderRegistration{{
				Source: Source{Kind: SourceKindExtension}, Provider: provider,
			}}, nil, nil, &testExecutor{})
			if err != nil {
				t.Fatalf("NewRegistry() error = %v", err)
			}
			healthy, err := service.Catalog(t.Context(), "ws-1", "")
			if err != nil {
				t.Fatalf("Catalog(healthy) error = %v", err)
			}
			provider.contribution.Commands[0].ProviderUnavailableReason = "extension notes is unhealthy (crash loop)"
			provider.contribution.Sources[0] = SourceStatus{
				Source: "ext.notes", Status: SourceUnhealthy, Reason: "extension notes is unhealthy (crash loop)",
			}
			provider.contribution.Defaults[0].Active = false
			unhealthy, err := service.Catalog(t.Context(), "ws-1", "")
			if err != nil {
				t.Fatalf("Catalog(unhealthy) error = %v", err)
			}
			if len(unhealthy.Commands) != 1 || unhealthy.Commands[0].Available ||
				unhealthy.Commands[0].UnavailableReason != "extension notes is unhealthy (crash loop)" ||
				unhealthy.Sources[0].Status != SourceUnhealthy || healthy.Revision == unhealthy.Revision {
				t.Fatalf("catalog transition = %#v -> %#v", healthy, unhealthy)
			}
			defaults, err := service.ExtensionDefaults(t.Context(), "ws-1")
			if err != nil || len(defaults) != 1 || defaults[0].Active {
				t.Fatalf("ExtensionDefaults() = %#v, %v", defaults, err)
			}
		},
	)

	t.Run("Should resolve bindings from the same extension snapshot [SI-5]", func(t *testing.T) {
		t.Parallel()
		descriptor := testDescriptor("ext.notes.capture")
		descriptor.Source = Source{Kind: SourceKindExtension, Extension: "notes"}
		provider := &countingContributionProvider{contribution: Contribution{
			Commands: []Descriptor{descriptor},
			Sources:  []SourceStatus{{Source: "ext.notes", Status: SourceHealthy}},
			Defaults: []ExtensionDefaultShortcut{{
				CommandID: descriptor.ID, Chord: "alt+shift+KeyN", Source: "ext.notes", Active: true,
			}},
		}}
		bindings := &snapshotTestBindings{}
		service, err := NewRegistry([]ProviderRegistration{{
			Source: Source{Kind: SourceKindExtension}, Provider: provider,
		}}, nil, bindings, &testExecutor{})
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		if _, err := service.Catalog(t.Context(), "ws-1", ""); err != nil {
			t.Fatalf("Catalog() error = %v", err)
		}
		if provider.calls != 1 || len(bindings.defaults) != 1 ||
			bindings.defaults[0].CommandID != descriptor.ID {
			t.Fatalf("snapshot reads = %d, defaults = %#v, want one atomic read", provider.calls, bindings.defaults)
		}
	})

	t.Run("Should publish catalog invalidation only to the canonical workspace", func(t *testing.T) {
		t.Parallel()
		recorder := &recordingEventRecorder{}
		now := time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC)
		service := testRegistryWithOptions(
			staticTestProvider{commands: []Descriptor{testDescriptor("core.test")}},
			nil,
			nil,
			&testExecutor{},
			WithEventRecorder(recorder),
			WithClock(func() time.Time { return now }),
		)
		workspaceEvents, cancelWorkspace, err := service.SubscribeCmdPaletteEvents(t.Context(), "ws-1")
		if err != nil {
			t.Fatalf("SubscribeCmdPaletteEvents(ws-1) error = %v", err)
		}
		defer cancelWorkspace()
		foreignEvents, cancelForeign, err := service.SubscribeCmdPaletteEvents(t.Context(), "ws-2")
		if err != nil {
			t.Fatalf("SubscribeCmdPaletteEvents(ws-2) error = %v", err)
		}
		defer cancelForeign()
		if err := service.NotifyCatalogChanged(t.Context(), "ws-1"); err != nil {
			t.Fatalf("NotifyCatalogChanged() error = %v", err)
		}
		event := <-workspaceEvents
		if event.Name != EventCatalogChanged || event.WorkspaceID != "ws-1" ||
			!strings.HasPrefix(event.CatalogRevision, "cr_") || !event.OccurredAt.Equal(now) {
			t.Fatalf("catalog event = %#v", event)
		}
		select {
		case foreign := <-foreignEvents:
			t.Fatalf("foreign workspace received event %#v", foreign)
		default:
		}
		if recorded := recorder.recorded(); len(recorded) != 1 || recorded[0] != event {
			t.Fatalf("recorded events = %#v, want %#v", recorded, event)
		}
	})
}

type contributionTestProvider struct {
	contribution Contribution
}

type countingContributionProvider struct {
	contribution Contribution
	calls        int
}

func (p *countingContributionProvider) ProvideCommands(context.Context, WorkspaceID) ([]Descriptor, error) {
	return cloneDescriptors(p.contribution.Commands), nil
}

func (p *countingContributionProvider) ProvideContribution(
	context.Context,
	WorkspaceID,
) (Contribution, error) {
	p.calls++
	return p.contribution, nil
}

type snapshotTestBindings struct {
	defaults []ExtensionDefaultShortcut
}

func (b *snapshotTestBindings) Bindings(
	context.Context,
	WorkspaceID,
) (map[CommandID][]string, map[CommandID]string, error) {
	return map[CommandID][]string{}, map[CommandID]string{}, nil
}

func (b *snapshotTestBindings) BindingsForCatalogSnapshot(
	_ context.Context,
	_ WorkspaceID,
	_ []CommandID,
	defaults []ExtensionDefaultShortcut,
) (map[CommandID][]string, map[CommandID]string, error) {
	b.defaults = append([]ExtensionDefaultShortcut(nil), defaults...)
	return map[CommandID][]string{}, map[CommandID]string{}, nil
}

func (p *contributionTestProvider) ProvideCommands(context.Context, WorkspaceID) ([]Descriptor, error) {
	return cloneDescriptors(p.contribution.Commands), nil
}

func (p *contributionTestProvider) ProvideContribution(context.Context, WorkspaceID) (Contribution, error) {
	result := p.contribution
	result.Commands = cloneDescriptors(p.contribution.Commands)
	result.Sources = append([]SourceStatus(nil), p.contribution.Sources...)
	result.Defaults = append([]ExtensionDefaultShortcut(nil), p.contribution.Defaults...)
	return result, nil
}
