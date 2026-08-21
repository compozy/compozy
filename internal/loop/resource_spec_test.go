package loop_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/resources"
)

func TestResourceSpecShouldProjectMetadataAndRejectInvalidDefinitions(t *testing.T) {
	t.Parallel()

	t.Run("Should project matchable metadata without definition body", func(t *testing.T) {
		t.Parallel()

		spec, _, err := loop.ParseResource([]byte(loopDefinitionYAML("catalog-loop")), loop.ResourceParseOptions{
			Source:   loop.SourceUser,
			Dir:      "/tmp/loops/catalog-loop",
			FilePath: "/tmp/loops/catalog-loop/loop.yaml",
		})
		if err != nil {
			t.Fatalf("ParseResource() error = %v", err)
		}
		if got, want := spec.Name, "catalog-loop"; got != want {
			t.Fatalf("spec.Name = %q, want %q", got, want)
		}
		if got, want := spec.Catalog.Category, "test"; got != want {
			t.Fatalf("spec.Catalog.Category = %q, want %q", got, want)
		}
		if got, want := spec.Catalog.Keywords, []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("spec.Catalog.Keywords = %#v, want %#v", got, want)
		}
		if got, want := spec.ContractGoal, "Test loop projection"; got != want {
			t.Fatalf("spec.ContractGoal = %q, want %q", got, want)
		}
		if got, want := len(spec.Start), 2; got != want {
			t.Fatalf("len(spec.Start) = %d, want %d", got, want)
		}
		if strings.Contains(spec.Description, "apiVersion") {
			t.Fatalf("spec.Description unexpectedly contains raw YAML: %q", spec.Description)
		}
		if spec.DefinitionChecksum == "" {
			t.Fatal("spec.DefinitionChecksum is empty, want content checksum")
		}
	})

	t.Run("Should reject a missing loop name", func(t *testing.T) {
		t.Parallel()

		body := strings.Replace(loopDefinitionYAML("missing-name"), "  name: missing-name\n", "", 1)
		_, _, err := loop.ParseResource([]byte(body), loop.ResourceParseOptions{
			Source:   loop.SourceUser,
			FilePath: "/tmp/loops/missing-name/loop.yaml",
		})
		requireResourceValidationErrorContains(t, err, "loop meta.name is required")
	})

	t.Run("Should reject unknown start kinds", func(t *testing.T) {
		t.Parallel()

		body := strings.Replace(loopDefinitionYAML("bad-start"), "  - kind: cli", "  - kind: unknown", 1)
		_, _, err := loop.ParseResource([]byte(body), loop.ResourceParseOptions{
			Source:   loop.SourceUser,
			FilePath: "/tmp/loops/bad-start/loop.yaml",
		})
		requireResourceValidationErrorContains(t, err, "loop start.kind is invalid")
	})

	t.Run("Should project tool actions without a schema source", func(t *testing.T) {
		t.Parallel()

		body := strings.Replace(
			loopDefinitionYAML("tool-action-loop"),
			"      kind: transform",
			"      kind: compozy__network_send",
			1,
		)
		body = strings.Replace(
			body,
			"concurrency: queue\n",
			"network_participation:\n  mode: live\n  channel_strategy: named\n  channel_id: builders\nconcurrency: queue\n",
			1,
		)
		_, _, err := loop.ParseResource([]byte(body), loop.ResourceParseOptions{
			Source:   loop.SourceUser,
			FilePath: "/tmp/loops/tool-action-loop/loop.yaml",
		})
		if err != nil {
			t.Fatalf("ParseResource(projection-only) error = %v", err)
		}

		_, _, err = loop.ParseResource([]byte(body), loop.ResourceParseOptions{
			Source:   loop.SourceUser,
			FilePath: "/tmp/loops/tool-action-loop/loop.yaml",
			Linter:   loop.NewLinter(),
		})
		requireResourceValidationErrorContains(t, err, loop.CodeUnknownActionKind)
		requireResourceValidationErrorContains(t, err, "without a schema snapshot source")
	})
}

func requireResourceValidationErrorContains(t *testing.T, err error, want string) {
	t.Helper()
	if !errors.Is(err, resources.ErrValidation) {
		t.Fatalf("ParseResource() error = %v, want resources.ErrValidation", err)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("ParseResource() error = %v, want to contain %q", err, want)
	}
}

func TestResourceSpecShouldResolveLoopPrecedence(t *testing.T) {
	t.Parallel()

	t.Run("Should prefer workspace records over user and marketplace records", func(t *testing.T) {
		t.Parallel()

		records := []resources.Record[loop.ResourceSpec]{
			loopRecord(
				"market",
				resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				loop.SourceMarketplace,
				2,
			),
			loopRecord(
				"user",
				resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
				loop.SourceUser,
				3,
			),
			loopRecord(
				"additional",
				resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
				loop.SourceAdditional,
				4,
			),
			loopRecord(
				"workspace",
				resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1"},
				loop.SourceWorkspace,
				5,
			),
			loopRecord(
				"other-workspace",
				resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: "ws-2"},
				loop.SourceWorkspace,
				6,
			),
		}
		for idx := range records {
			records[idx].Spec.Name = "delivery-loop"
		}

		resolved := loop.ResolveEffectiveResources(records, loop.ResourceLens{WorkspaceID: "ws-1"})
		if got, want := len(resolved), 1; got != want {
			t.Fatalf("len(ResolveEffectiveResources()) = %d, want %d", got, want)
		}
		if got, want := resolved[0].ID, "workspace"; got != want {
			t.Fatalf("resolved[0].ID = %q, want %q", got, want)
		}

		unscoped := loop.ResolveEffectiveResources(records, loop.ResourceLens{})
		if got, want := unscoped[0].ID, "user"; got != want {
			t.Fatalf("unscoped resolved ID = %q, want %q", got, want)
		}
	})

	t.Run("Should isolate profile layers and apply four-layer precedence", func(t *testing.T) {
		t.Parallel()

		records := []resources.Record[loop.ResourceSpec]{
			loopRecord("user", resources.ResourceScope{Kind: resources.ResourceScopeKindUser}, loop.SourceWorkspace, 1),
			loopRecord("marketing-profile", resources.ResourceScope{
				Kind: resources.ResourceScopeKindProfile, ID: "profile-marketing",
			}, loop.SourceMarketplace, 1),
			loopRecord("engineering-profile", resources.ResourceScope{
				Kind: resources.ResourceScopeKindProfile, ID: "profile-engineering",
			}, loop.SourceWorkspace, 1),
			loopRecord("workspace", resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspace, ID: "ws-1",
			}, loop.SourceMarketplace, 1),
			loopRecord("marketing-workspace-profile", resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "ws-1@pf:marketing",
			}, loop.SourceMarketplace, 1),
			loopRecord("engineering-workspace-profile", resources.ResourceScope{
				Kind: resources.ResourceScopeKindWorkspaceProfile, ID: "ws-1@pf:engineering",
			}, loop.SourceWorkspace, 1),
		}
		for idx := range records {
			records[idx].Spec.Name = "delivery-loop"
		}

		marketing := loop.ResolveEffectiveResources(records, loop.ResourceLens{
			WorkspaceID: "ws-1", ProfileID: "profile-marketing", ProfileName: "marketing",
		})
		if got, want := marketing[0].ID, "marketing-workspace-profile"; got != want {
			t.Fatalf("marketing resolved ID = %q, want %q", got, want)
		}

		engineering := loop.ResolveEffectiveResources(records, loop.ResourceLens{
			WorkspaceID: "ws-1", ProfileID: "profile-engineering", ProfileName: "engineering",
		})
		if got, want := engineering[0].ID, "engineering-workspace-profile"; got != want {
			t.Fatalf("engineering resolved ID = %q, want %q", got, want)
		}
	})
}

func loopRecord(
	id string,
	scope resources.ResourceScope,
	source loop.Source,
	version int64,
) resources.Record[loop.ResourceSpec] {
	return resources.Record[loop.ResourceSpec]{
		ID:      id,
		Version: version,
		Scope:   scope,
		Spec: loop.ResourceSpec{
			Name:     id,
			Source:   source,
			FilePath: "/tmp/" + id + "/loop.yaml",
		},
	}
}

func loopDefinitionYAML(name string) string {
	return `apiVersion: compozy.loop/v1
kind: Loop
meta:
  name: ` + name + `
  version: 1
  description: Test loop
  catalog:
    use_when: Test catalog projection
    keywords: [beta, alpha, beta]
    category: test
concurrency: queue
inputs:
  target:
    type: string
    required: true
contract:
  goal: Test loop projection
  definition_of_done: Projection works
  terminal_states: [done, failed]
  iteration_cap: 3
  no_progress:
    window: 2
  budget:
    tokens: 0
    wall_clock_sec: 0
    on_exceeded: halt
start:
  - kind: cli
  - kind: native_tool
graph:
  nodes:
    - id: target_input
      class: source
      kind: input
      input_ref: target
    - id: normalize_target
      class: action
      kind: transform
      params:
        map:
          target:
            value: ok
  edges:
    - from: target_input
      to: normalize_target
`
}
