package cli

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func TestBuildCommandInputProjectsTypedFlags(t *testing.T) {
	t.Parallel()

	t.Run("Should convert scalars arrays and omit absent optional fields", func(t *testing.T) {
		t.Parallel()

		descriptor := commandCLIDescriptor()
		flags := newProjectedCommandFlagSet(descriptor)
		if err := flags.Parse([]string{
			"--name", "my-feature",
			"--round", "2",
			"--ratio", "1.5",
			"--enabled",
			"--tag", "alpha",
			"--tag", "beta",
		}); err != nil {
			t.Fatalf("FlagSet.Parse() error = %v", err)
		}
		input, err := buildCommandInput(descriptor, flags, "")
		if err != nil {
			t.Fatalf("buildCommandInput() error = %v", err)
		}
		want := `{"enabled":true,"name":"my-feature","ratio":1.5,"round":2,"tags":["alpha","beta"]}`
		if string(input) != want {
			t.Fatalf("buildCommandInput() = %s, want %s", input, want)
		}
		if strings.Contains(string(input), "optional") {
			t.Fatalf("buildCommandInput() injected absent optional field: %s", input)
		}
	})

	t.Run("Should preserve explicit false for a boolean flag", func(t *testing.T) {
		t.Parallel()

		descriptor := commandCLIDescriptor()
		flags := newProjectedCommandFlagSet(descriptor)
		if err := flags.Parse([]string{"--name=x", "--round=1", "--enabled=false"}); err != nil {
			t.Fatalf("FlagSet.Parse() error = %v", err)
		}
		input, err := buildCommandInput(descriptor, flags, "")
		if err != nil {
			t.Fatalf("buildCommandInput() error = %v", err)
		}
		if string(input) != `{"enabled":false,"name":"x","round":1}` {
			t.Fatalf("buildCommandInput() = %s", input)
		}
	})

	t.Run("Should name both flag and field when conversion fails", func(t *testing.T) {
		t.Parallel()

		descriptor := commandCLIDescriptor()
		flags := newProjectedCommandFlagSet(descriptor)
		if err := flags.Parse([]string{"--name=x", "--round=abc"}); err != nil {
			t.Fatalf("FlagSet.Parse() error = %v", err)
		}
		_, err := buildCommandInput(descriptor, flags, "")
		if err == nil || !strings.Contains(err.Error(), "--round") || !strings.Contains(err.Error(), `"round"`) {
			t.Fatalf("buildCommandInput() error = %v, want flag and field", err)
		}
	})

	t.Run("Should reject missing required projected flags", func(t *testing.T) {
		t.Parallel()

		descriptor := commandCLIDescriptor()
		flags := newProjectedCommandFlagSet(descriptor)
		_, err := buildCommandInput(descriptor, flags, "")
		if err == nil || !strings.Contains(err.Error(), "--name") || !strings.Contains(err.Error(), `"name"`) {
			t.Fatalf("buildCommandInput() error = %v, want required flag diagnostic", err)
		}
	})

	t.Run("Should reject raw input combined with projected flags", func(t *testing.T) {
		t.Parallel()

		descriptor := commandCLIDescriptor()
		flags := newProjectedCommandFlagSet(descriptor)
		if err := flags.Parse([]string{"--name=x"}); err != nil {
			t.Fatalf("FlagSet.Parse() error = %v", err)
		}
		_, err := buildCommandInput(descriptor, flags, `{"name":"raw"}`)
		if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
			t.Fatalf("buildCommandInput() error = %v, want mutual exclusion", err)
		}
	})

	t.Run("Should pass raw JSON through canonical parsing without injecting defaults", func(t *testing.T) {
		t.Parallel()

		descriptor := commandCLIDescriptor()
		flags := newProjectedCommandFlagSet(descriptor)
		input, err := buildCommandInput(descriptor, flags, `{ "name": "raw" }`)
		if err != nil {
			t.Fatalf("buildCommandInput() error = %v", err)
		}
		if string(input) != `{"name":"raw"}` {
			t.Fatalf("buildCommandInput() = %s", input)
		}
	})
}

func TestExtensionCommandSelectionRejectsGroupsAndUnknownPaths(t *testing.T) {
	t.Parallel()

	catalog := commandCLICatalog()
	tests := []struct {
		name     string
		path     string
		contains []string
	}{
		{
			name:     "Should reject a presentation group and list its leaves",
			path:     "review",
			contains: []string{"not executable", "review/fetch"},
		},
		{
			name:     "Should reject an unknown path with available leaves and a suggestion",
			path:     "review/fetc",
			contains: []string{"available commands", "greet", "review/fetch", "did you mean"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := selectExtensionCommand(catalog, "cmd-fixture", tt.path)
			if err == nil {
				t.Fatal("selectExtensionCommand() error = nil")
			}
			for _, fragment := range tt.contains {
				if !strings.Contains(err.Error(), fragment) {
					t.Fatalf("selectExtensionCommand() error = %v, want %q", err, fragment)
				}
			}
		})
	}
}

func TestExtensionCommandsRendersHumanTreeAndStructuredLeaves(t *testing.T) {
	t.Parallel()

	t.Run("Should render the declared group summary and nested leaf", func(t *testing.T) {
		t.Parallel()

		client := commandCLIClient(t, nil)
		stdout, _, err := executeRootCommand(
			t,
			commandCLITestDeps(t, client),
			"extension", "commands", "cmd-fixture", "--workspace", "ws-1",
		)
		if err != nil {
			t.Fatalf("extension commands error = %v", err)
		}
		for _, fragment := range []string{"review/", "Review commands", "review/fetch", "ext__cmd_fixture__review_fetch"} {
			if !strings.Contains(stdout, fragment) {
				t.Fatalf("extension commands output = %q, want %q", stdout, fragment)
			}
		}
	})

	t.Run("Should emit a flat command array with safety and flag metadata", func(t *testing.T) {
		t.Parallel()

		client := commandCLIClient(t, nil)
		stdout, _, err := executeRootCommand(
			t,
			commandCLITestDeps(t, client),
			"extension", "commands", "cmd-fixture", "--workspace", "ws-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("extension commands error = %v", err)
		}
		var commands []contract.ExtensionCommandPayload
		decodeJSONOutput(t, stdout, &commands)
		if len(commands) != 2 || commands[0].Verb != "greet" || commands[1].Verb != "review/fetch" {
			t.Fatalf("structured commands = %#v", commands)
		}
		if commands[0].RiskClass != toolspkg.RiskMutating ||
			!commands[0].ApprovalRequired ||
			len(commands[0].Flags) == 0 {
			t.Fatalf("structured greet metadata = %#v", commands[0])
		}
	})
}

func TestExtensionExecPreservesHostScopeAndApprovalMetadata(t *testing.T) {
	t.Parallel()

	t.Run("Should forward scope token and one typed input to the canonical invoke", func(t *testing.T) {
		t.Parallel()

		invokeCount := 0
		client := commandCLIClient(t, func(id string, request ToolInvokeRequest) {
			invokeCount++
			if id != "ext__cmd_fixture__greet" {
				t.Fatalf("InvokeTool() id = %q", id)
			}
			if request.WorkspaceID != "ws-1" ||
				request.SessionID != "sess-1" ||
				request.AgentName != "coder" {
				t.Fatalf("InvokeTool() scope = %#v", request)
			}
			if request.ApprovalToken != "approval-ref" {
				t.Fatalf("InvokeTool() approval token = %q", request.ApprovalToken)
			}
			if string(request.Input) != `{"name":"Ada","round":2}` {
				t.Fatalf("InvokeTool() input = %s", request.Input)
			}
		})
		_, _, err := executeRootCommand(
			t,
			commandCLITestDeps(t, client),
			"extension", "exec", "cmd-fixture",
			"--cmd", "greet",
			"--name", "Ada",
			"--round", "2",
			"--workspace", "ws-1",
			"--session", "sess-1",
			"--agent", "coder",
			"--approval-token", "approval-ref",
		)
		if err != nil {
			t.Fatalf("extension exec error = %v", err)
		}
		if invokeCount != 1 {
			t.Fatalf("InvokeTool() calls = %d, want 1", invokeCount)
		}
	})

	t.Run("Should stop before invocation when the selected path is a group", func(t *testing.T) {
		t.Parallel()

		invokeCount := 0
		client := commandCLIClient(t, func(string, ToolInvokeRequest) { invokeCount++ })
		_, _, err := executeRootCommand(
			t,
			commandCLITestDeps(t, client),
			"extension", "exec", "cmd-fixture", "--cmd", "review", "--workspace", "ws-1",
		)
		if err == nil || !strings.Contains(err.Error(), "review/fetch") {
			t.Fatalf("extension exec group error = %v", err)
		}
		if invokeCount != 0 {
			t.Fatalf("InvokeTool() calls = %d, want 0", invokeCount)
		}
	})
}

func commandCLIDescriptor() contract.ExtensionCommandPayload {
	minimum, maximum := float64(1), float64(10)
	return contract.ExtensionCommandPayload{
		Extension: "cmd-fixture", Verb: "greet", ToolID: "ext__cmd_fixture__greet",
		Summary: "Greet a person", RiskClass: toolspkg.RiskMutating, ApprovalRequired: true,
		Flags: []toolspkg.CommandFlag{
			{Name: "enabled", Field: "enabled", Type: toolspkg.CommandFlagBoolean},
			{Name: "name", Field: "name", Type: toolspkg.CommandFlagString, Required: true},
			{
				Name: "optional", Field: "optional", Type: toolspkg.CommandFlagString,
				Default: json.RawMessage(`"default"`),
			},
			{Name: "ratio", Field: "ratio", Type: toolspkg.CommandFlagNumber},
			{
				Name: "round", Field: "round", Type: toolspkg.CommandFlagInteger,
				Required: true, Minimum: &minimum, Maximum: &maximum,
			},
			{
				Name: "tag", Field: "tags", Type: toolspkg.CommandFlagString,
				Repeatable: true, Enum: []string{"alpha", "beta"},
			},
		},
	}
}

func commandCLICatalog() ExtensionCommandsRecord {
	greet := commandCLIDescriptor()
	nested := contract.ExtensionCommandPayload{
		Extension: "cmd-fixture", Verb: "review/fetch", ToolID: "ext__cmd_fixture__review_fetch",
		Summary: "Fetch review", RiskClass: toolspkg.RiskRead,
	}
	return ExtensionCommandsRecord{
		Commands: []contract.ExtensionCommandPayload{greet, nested},
		Groups: []contract.ExtensionCommandGroupPayload{
			{Extension: "cmd-fixture", Path: "review", Summary: "Review commands"},
		},
	}
}

func commandCLIClient(t *testing.T, invoked func(string, ToolInvokeRequest)) *stubClient {
	t.Helper()
	return &stubClient{
		listExtensionCommandsFn: func(_ context.Context, extension, workspaceID string) (ExtensionCommandsRecord, error) {
			if extension != "cmd-fixture" || workspaceID != "ws-1" {
				t.Fatalf("ListExtensionCommands(%q, %q)", extension, workspaceID)
			}
			return commandCLICatalog(), nil
		},
		invokeToolFn: func(_ context.Context, id string, request ToolInvokeRequest) (ToolInvokeResponseRecord, error) {
			if invoked != nil {
				invoked(id, request)
			}
			response := sampleInvokeResponse()
			response.ToolID = toolspkg.ToolID(id)
			return response, nil
		},
	}
}

func commandCLITestDeps(t *testing.T, client DaemonClient) commandDeps {
	t.Helper()
	deps := newWorkspaceTestDeps(t, client)
	markExtensionDaemonRunning(&deps)
	return deps
}
