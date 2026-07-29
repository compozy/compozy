package extensionpkg

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
	extensionprotocol "github.com/compozy/compozy/internal/extensionprotocol"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestCommandProjectionMaterializesClosedFlagContract(t *testing.T) {
	t.Parallel()

	t.Run("Should project complete flag metadata in lexical order", func(t *testing.T) {
		t.Parallel()

		manifest := commandTestManifest("cmd-fixture")
		commands, groups, err := projectExtensionCommands(manifest)
		if err != nil {
			t.Fatalf("projectExtensionCommands() error = %v", err)
		}
		if len(groups) != 1 || groups[0].Path != "review" || groups[0].Summary != "Review changes" {
			t.Fatalf("projectExtensionCommands() groups = %#v, want declared review group", groups)
		}
		if got, want := len(commands), 2; got != want {
			t.Fatalf("len(projectExtensionCommands()) = %d, want %d", got, want)
		}
		greet := commands[0]
		if greet.Verb != "greet" || greet.ToolID != "ext__cmd_fixture__greet" {
			t.Fatalf("greet descriptor = %#v", greet)
		}
		if greet.RiskClass != toolspkg.RiskMutating || !greet.ApprovalRequired {
			t.Fatalf("greet safety metadata = risk %q approval %v", greet.RiskClass, greet.ApprovalRequired)
		}
		minimum, maximum := float64(1), float64(10)
		wantFlags := []extensioncontract.CommandFlag{
			{
				Name: "count", Field: "count", Type: extensioncontract.CommandFlagInteger,
				Required: true, Default: json.RawMessage(`2`), Minimum: &minimum, Maximum: &maximum,
			},
			{
				Name: "name", Field: "name", Type: extensioncontract.CommandFlagString,
				Nullable: true, Enum: []string{"alpha", "beta"}, Default: json.RawMessage(`"alpha"`),
			},
			{
				Name: "tag", Field: "tags", Type: extensioncontract.CommandFlagString,
				Repeatable: true,
			},
		}
		if !reflect.DeepEqual(greet.Flags, wantFlags) {
			t.Fatalf("greet flags = %#v, want %#v", greet.Flags, wantFlags)
		}
	})
}

func TestCommandDeclarationValidation(t *testing.T) {
	t.Parallel()

	reserved := []string{"cmd", "input", "output", "o", "json", "help"}
	for _, flagName := range reserved {
		t.Run("Should reject reserved flag "+flagName, func(t *testing.T) {
			t.Parallel()
			manifest := commandTestManifest("cmd-fixture")
			command := manifest.Resources.Tools["greet"].Command
			command.Flags = map[string]string{flagName: "name"}
			assertCommandRejectedAtBuildAndLoad(t, manifest, flagName, "reserved flags")
		})
	}

	tests := []struct {
		name     string
		mutate   func(*Manifest)
		contains []string
	}{
		{
			name: "Should reject a flag mapped to a missing top-level field",
			mutate: func(manifest *Manifest) {
				manifest.Resources.Tools["greet"].Command.Flags = map[string]string{"ghost": "missing_field"}
			},
			contains: []string{"ghost", "missing_field"},
		},
		{
			name: "Should reject a duplicate leaf path",
			mutate: func(manifest *Manifest) {
				tool := manifest.Resources.Tools["review_fetch"]
				tool.Command.Verb = "greet"
				manifest.Resources.Tools["review_fetch"] = tool
			},
			contains: []string{"greet", "duplicate command path"},
		},
		{
			name: "Should reject a leaf deeper than two segments",
			mutate: func(manifest *Manifest) {
				manifest.Resources.Tools["review_fetch"].Command.Verb = "review/round/fetch"
			},
			contains: []string{"review/round/fetch", "at most 2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manifest := commandTestManifest("cmd-fixture")
			tt.mutate(manifest)
			assertCommandRejectedAtBuildAndLoad(t, manifest, tt.contains...)
		})
	}

	t.Run("Should warn when nested and flat paths are visually ambiguous", func(t *testing.T) {
		t.Parallel()

		manifest := commandTestManifest("cmd-fixture")
		manifest.Resources.Tools["review_flat"] = commandTestTool(
			"review_flat",
			"review-fetch",
			json.RawMessage(`{"type":"object","properties":{}}`),
			nil,
			false,
		)
		data, err := encodeManifestTOML(manifest)
		if err != nil {
			t.Fatalf("encodeManifestTOML() error = %v", err)
		}
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, manifestTOMLFileName), string(data))
		loaded, issues, err := ValidateBundle(dir)
		if err != nil {
			t.Fatalf("ValidateBundle() error = %v", err)
		}
		if loaded == nil || len(issues) != 1 || issues[0].Severity != IssueSeverityWarning ||
			!strings.Contains(issues[0].Message, "review/fetch") ||
			!strings.Contains(issues[0].Message, "review-fetch") {
			t.Fatalf("ValidateBundle() issues = %#v", issues)
		}
	})
}

func TestCommandGroupValidationAndRoundTrip(t *testing.T) {
	t.Parallel()

	t.Run("Should reject every malformed declared group at build and load", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name     string
			mutate   func(*Manifest)
			contains string
		}{
			{
				name: "Should reject a group colliding with a flat leaf",
				mutate: func(manifest *Manifest) {
					manifest.Resources.CommandGroups[0].Path = "greet"
				},
				contains: "greet",
			},
			{
				name: "Should reject an executable leaf used as a nested parent",
				mutate: func(manifest *Manifest) {
					tool := manifest.Resources.Tools["greet"]
					tool.Command.Verb = "review"
					manifest.Resources.Tools["greet"] = tool
				},
				contains: "review",
			},
			{
				name: "Should reject an empty path segment",
				mutate: func(manifest *Manifest) {
					manifest.Resources.CommandGroups[0].Path = "review//x"
				},
				contains: "review//x",
			},
			{
				name:     "Should reject a leading slash",
				mutate:   func(manifest *Manifest) { manifest.Resources.CommandGroups[0].Path = "/review" },
				contains: "/review",
			},
			{
				name:     "Should reject a trailing slash",
				mutate:   func(manifest *Manifest) { manifest.Resources.CommandGroups[0].Path = "review/" },
				contains: "review/",
			},
			{
				name: "Should reject duplicate declarations",
				mutate: func(manifest *Manifest) {
					manifest.Resources.CommandGroups = append(
						manifest.Resources.CommandGroups,
						extensioncontract.ExtensionCommandGroupSpec{Path: "review", Summary: "Again"},
					)
				},
				contains: "duplicate command group",
			},
			{
				name:     "Should reject a multi-segment group",
				mutate:   func(manifest *Manifest) { manifest.Resources.CommandGroups[0].Path = "review/rounds" },
				contains: "review/rounds",
			},
			{
				name:     "Should reject a leafless group",
				mutate:   func(manifest *Manifest) { manifest.Resources.CommandGroups[0].Path = "missing" },
				contains: "no leaf",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				manifest := commandTestManifest("cmd-fixture")
				tt.mutate(manifest)
				assertCommandRejectedAtBuildAndLoad(t, manifest, tt.contains)
			})
		}
	})

	t.Run("Should preserve declared summaries and synthesize implicit prefix groups", func(t *testing.T) {
		t.Parallel()

		manifest := commandTestManifest("cmd-fixture")
		manifest.Resources.Tools["sync_run"] = commandTestTool(
			"sync_run",
			"sync/run",
			json.RawMessage(`{"type":"object","properties":{}}`),
			nil,
			false,
		)
		built, err := manifestFromDescribe(commandDescribePayload(t, manifest))
		if err != nil {
			t.Fatalf("manifestFromDescribe() error = %v", err)
		}
		data, err := encodeManifestTOML(built)
		if err != nil {
			t.Fatalf("encodeManifestTOML() error = %v", err)
		}
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, manifestTOMLFileName), string(data))
		loaded, err := LoadManifest(dir)
		if err != nil {
			t.Fatalf("LoadManifest() error = %v", err)
		}
		commands, groups, err := projectExtensionCommands(loaded)
		if err != nil {
			t.Fatalf("projectExtensionCommands() error = %v", err)
		}
		if got, want := len(commands), 3; got != want {
			t.Fatalf("len(commands) = %d, want %d", got, want)
		}
		wantGroups := []CommandGroupDescriptor{
			{Extension: "cmd-fixture", Path: "review", Summary: "Review changes"},
			{Extension: "cmd-fixture", Path: "sync"},
		}
		slices.SortFunc(groups, compareCommandGroupDescriptors)
		if !reflect.DeepEqual(groups, wantGroups) {
			t.Fatalf("groups = %#v, want %#v", groups, wantGroups)
		}
		if commands[0].Verb != "greet" || !commands[0].ApprovalRequired {
			t.Fatalf("TOML round-trip command metadata = %#v, want approval required", commands[0])
		}
	})
}

func TestCommandProjectionRejectsUnsupportedSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		schema string
	}{
		{name: "Should reject object fields", schema: `{"type":"object","properties":{}}`},
		{
			name:   "Should reject nested arrays",
			schema: `{"type":"array","items":{"type":"array","items":{"type":"string"}}}`,
		},
		{name: "Should reject tuple arrays", schema: `{"type":"array","prefixItems":[{"type":"string"}]}`},
		{name: "Should reject oneOf", schema: `{"oneOf":[{"type":"string"},{"type":"number"}]}`},
		{name: "Should reject anyOf", schema: `{"anyOf":[{"type":"string"},{"type":"number"}]}`},
		{name: "Should reject allOf", schema: `{"allOf":[{"type":"string"}]}`},
		{name: "Should reject not", schema: `{"type":"string","not":{"const":"x"}}`},
		{name: "Should reject multi-type unions", schema: `{"type":["string","number"]}`},
		{name: "Should reject unresolved references", schema: `{"$ref":"#/$defs/missing"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			manifest := commandTestManifest("cmd-fixture")
			manifest.Resources.Tools["greet"] = commandTestTool(
				"greet",
				"greet",
				json.RawMessage(`{"type":"object","properties":{"value":`+tt.schema+`}}`),
				map[string]string{"value": "value"},
				false,
			)
			assertCommandRejectedAtBuildAndLoad(t, manifest, "value", "--input")
		})
	}
}

func TestManagerCommandsFiltersUnavailableExtensionsAndSortsDeterministically(t *testing.T) {
	t.Parallel()

	t.Run("Should expose only active healthy enabled extensions", func(t *testing.T) {
		t.Parallel()

		manager := NewManager(nil)
		for _, state := range []struct {
			name       string
			enabled    bool
			registered bool
			active     bool
		}{
			{name: "active", enabled: true, registered: true, active: true},
			{name: "disabled", registered: true, active: true},
			{name: "inactive", enabled: true, registered: true},
			{name: "unregistered", enabled: true, active: true},
		} {
			manifest := commandTestManifest(state.name)
			manager.extensions[state.name] = &managedExtension{
				key:      GlobalInstanceKey(state.name),
				info:     ExtensionInfo{Name: state.name, Version: "0.1.0", Enabled: state.enabled},
				manifest: manifest, registered: state.registered, active: state.active,
			}
		}

		commands, groups, err := manager.Commands("")
		if err != nil {
			t.Fatalf("Manager.Commands() error = %v", err)
		}
		if len(commands) != 2 || commands[0].Extension != "active" || commands[1].Extension != "active" {
			t.Fatalf("Manager.Commands() commands = %#v, want active extension only", commands)
		}
		if len(groups) != 1 || groups[0].Extension != "active" || groups[0].Path != "review" {
			t.Fatalf("Manager.Commands() groups = %#v, want active review group", groups)
		}
	})
}

func commandTestManifest(name string) *Manifest {
	return &Manifest{
		Name: name, Version: "0.1.0", MinCompozyVersion: "0.3.0-beta.1",
		Capabilities: CapabilitiesConfig{Provides: []string{extensionprotocol.CapabilityToolProvider}},
		Subprocess:   SubprocessConfig{Command: "./cmd-fixture"},
		Resources: ResourcesConfig{
			CommandGroups: []extensioncontract.ExtensionCommandGroupSpec{
				{Path: "review", Summary: "Review changes"},
			},
			Tools: map[string]ToolConfig{
				"greet": commandTestTool(
					"greet",
					"greet",
					json.RawMessage(`{
						"type":"object",
						"properties":{
							"count":{"type":"integer","default":2,"minimum":1,"maximum":10},
							"name":{"type":["string","null"],"enum":["alpha","beta"],"default":"alpha"},
							"tags":{"type":"array","items":{"type":"string"}}
						},
						"required":["count"]
					}`),
					map[string]string{"count": "count", "name": "name", "tag": "tags"},
					true,
				),
				"review_fetch": commandTestTool(
					"review_fetch",
					"review/fetch",
					json.RawMessage(`{"type":"object","properties":{"round":{"type":"integer"}}}`),
					map[string]string{"round": "round"},
					false,
				),
			},
		},
	}
}

func commandTestTool(
	handler string,
	verb string,
	input json.RawMessage,
	flags map[string]string,
	requiresInteraction bool,
) ToolConfig {
	return ToolConfig{
		Description:         "Fixture command " + verb,
		Handler:             handler,
		Backend:             ToolBackendConfig{Kind: string(toolspkg.BackendExtensionHost), Handler: handler},
		InputSchema:         input,
		Risk:                string(toolspkg.RiskMutating),
		Visibility:          string(toolspkg.VisibilityModel),
		RequiresInteraction: requiresInteraction,
		Command: &toolspkg.ExtensionCommandSpec{
			Verb: verb, Summary: "Run " + verb, Example: "--example", Flags: flags,
		},
	}
}

func commandDescribePayload(t *testing.T, manifest *Manifest) extensioncontract.DescribePayload {
	t.Helper()

	descriptors, err := ResolveManifestToolDescriptors(manifest)
	if err != nil {
		t.Fatalf("ResolveManifestToolDescriptors() error = %v", err)
	}
	runtime := make([]toolspkg.ExtensionToolRuntimeDescriptor, 0, len(descriptors))
	for index := range descriptors {
		descriptor := &descriptors[index]
		current := descriptor.RuntimeDescriptor
		config := manifest.Resources.Tools[descriptor.Name]
		current.Description = config.Description
		current.InputSchema = cloneRawMessage(config.InputSchema)
		current.OutputSchema = cloneRawMessage(config.OutputSchema)
		runtime = append(runtime, current)
	}
	return extensioncontract.DescribePayload{
		Name: manifest.Name, Version: manifest.Version, Description: manifest.Description,
		Provides: manifest.Capabilities.Provides, Permissions: []string{},
		Subprocess:    extensioncontract.DescribeSubprocess{Command: manifest.Subprocess.Command},
		Tools:         runtime,
		CommandGroups: cloneCommandGroups(manifest.Resources.CommandGroups),
		SDK: extensioncontract.DescribeSDKInfo{
			Name: "test-sdk", Version: "0.1.0", ProtocolVersion: "1", MinCompozyVersion: manifest.MinCompozyVersion,
		},
	}
}

func assertCommandRejectedAtBuildAndLoad(t *testing.T, manifest *Manifest, fragments ...string) {
	t.Helper()

	_, buildErr := manifestFromDescribe(commandDescribePayload(t, manifest))
	assertCommandValidationError(t, "build", buildErr, fragments...)
	assertCommandValidationError(t, "load", manifest.Validate(), fragments...)
}

func assertCommandValidationError(t *testing.T, lane string, err error, fragments ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s validation error = nil", lane)
	}
	for _, fragment := range fragments {
		if !strings.Contains(err.Error(), fragment) {
			t.Fatalf("%s validation error = %v, want fragment %q", lane, err, fragment)
		}
	}
}
