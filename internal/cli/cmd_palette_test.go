package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
)

type cmdPaletteTestClient struct {
	DaemonClient
	commands       contract.CmdPaletteCommandsResponse
	invokeResult   contract.CmdPaletteInvokeResult
	invokeErr      error
	invokeCommand  string
	invokeRequest  contract.CmdPaletteInvokeRequest
	listWorkspace  string
	listClient     string
	approvalStatus contract.ToolApprovalStatusResponse
	canceledID     string
}

func (c *cmdPaletteTestClient) ListCmdPaletteCommands(
	_ context.Context,
	workspace string,
	client string,
) (contract.CmdPaletteCommandsResponse, error) {
	c.listWorkspace = workspace
	c.listClient = client
	return c.commands, nil
}

func (c *cmdPaletteTestClient) ListCmdPaletteClients(
	context.Context,
	string,
) ([]contract.CmdPaletteClient, error) {
	return nil, nil
}

func (c *cmdPaletteTestClient) InvokeCmdPaletteCommand(
	_ context.Context,
	commandID string,
	request contract.CmdPaletteInvokeRequest,
) (contract.CmdPaletteInvokeResult, error) {
	c.invokeCommand = commandID
	c.invokeRequest = request
	return c.invokeResult, c.invokeErr
}

func (c *cmdPaletteTestClient) GetPendingToolApproval(
	context.Context,
	string,
) (contract.ToolApprovalStatusResponse, error) {
	return c.approvalStatus, nil
}

func (c *cmdPaletteTestClient) CancelPendingToolApproval(
	_ context.Context,
	approvalID string,
) (contract.ToolApprovalStatusResponse, error) {
	c.canceledID = approvalID
	return c.approvalStatus, nil
}

func TestCmdPaletteCommands(t *testing.T) {
	t.Parallel()

	newClient := func() *cmdPaletteTestClient {
		return &cmdPaletteTestClient{
			DaemonClient: withWorkspaceResolution(&stubClient{}),
			commands: contract.CmdPaletteCommandsResponse{Commands: []contract.CmdPaletteCommand{
				{ID: "ext.demo.hidden", Source: "ext.demo", Available: false},
				{ID: "core.sessions.new", Source: "core", Available: true},
				{ID: "core.sessions.stop", Source: "core", Available: false},
			}},
		}
	}

	t.Run("Should resolve workspace and apply catalog filters", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "list", "--workspace", "workspace-1", "--source", "core", "--available=true", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette list error = %v", err)
		}
		var commands []contract.CmdPaletteCommand
		if err := json.Unmarshal([]byte(stdout), &commands); err != nil {
			t.Fatalf("json.Unmarshal(list output) error = %v", err)
		}
		if len(commands) != 1 || commands[0].ID != "core.sessions.new" {
			t.Fatalf("commands = %#v, want only core.sessions.new", commands)
		}
	})

	t.Run("Should parse typed arguments and return pending approval", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.commands.Commands = []contract.CmdPaletteCommand{{
			ID: "core.tools.run", Arguments: []cmdpalette.Argument{{Name: "force", Type: cmdpalette.ArgumentTypeCheckbox}},
		}}
		client.invokeResult = contract.CmdPaletteInvokeResult{
			Status: cmdpalette.InvokeStatusApprovalPending, ApprovalID: "approval-1",
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "invoke", "core.tools.run", "--workspace", "workspace-1", "--arg", "force=true", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette invoke error = %v", err)
		}
		if client.invokeCommand != "core.tools.run" || client.invokeRequest.Workspace != "workspace-1" {
			t.Fatalf("invoke command/request = %q/%#v", client.invokeCommand, client.invokeRequest)
		}
		if force, ok := client.invokeRequest.Args["force"].(bool); !ok || !force {
			t.Fatalf("invoke force = %#v, want true", client.invokeRequest.Args["force"])
		}
		var output cmdPaletteInvokeOutputRecord
		if err := json.Unmarshal([]byte(stdout), &output); err != nil {
			t.Fatalf("json.Unmarshal(invoke output) error = %v", err)
		}
		if output.Status != cmdpalette.InvokeStatusApprovalPending || output.ApprovalID != "approval-1" {
			t.Fatalf("invoke output = %#v", output)
		}
	})

	t.Run("Should map invocation validation failures to exit code two", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.commands.Commands = []contract.CmdPaletteCommand{{ID: "core.invalid"}}
		client.invokeErr = &daemonAPIError{statusCode: http.StatusUnprocessableEntity, status: "422 Unprocessable Entity"}
		exitCode, _, _ := executeRootCommandWithExit(
			t,
			newTestDeps(t, client),
			"cmd-palette", "invoke", "core.invalid", "--workspace", "workspace-1", "-o", "json",
		)
		if exitCode != 2 {
			t.Fatalf("invoke exit code = %d, want 2", exitCode)
		}
	})

	t.Run("Should reject invalid checkbox arguments before invocation", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.commands.Commands = []contract.CmdPaletteCommand{{
			ID: "core.tools.run", Arguments: []cmdpalette.Argument{{Name: "force", Type: cmdpalette.ArgumentTypeCheckbox}},
		}}
		exitCode, _, _ := executeRootCommandWithExit(
			t,
			newTestDeps(t, client),
			"cmd-palette", "invoke", "core.tools.run", "--workspace", "workspace-1", "--arg", "force=maybe",
		)
		if exitCode != 2 || client.invokeCommand != "" {
			t.Fatalf("invalid argument exit/invocation = %d/%q, want 2/no invocation", exitCode, client.invokeCommand)
		}
	})

	t.Run("Should cancel the exact approval ID", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.approvalStatus.ApprovalStatus = "denied"
		if _, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"approvals", "cancel", "approval-1", "-o", "json",
		); err != nil {
			t.Fatalf("approvals cancel error = %v", err)
		}
		if client.canceledID != "approval-1" {
			t.Fatalf("canceled approval = %q, want approval-1", client.canceledID)
		}
	})

	t.Run("Should stream every filtered command as one JSONL record [IT-001][E2E-022]", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.commands.Commands = make([]contract.CmdPaletteCommand, 0, 512)
		for index := range 512 {
			client.commands.Commands = append(client.commands.Commands, contract.CmdPaletteCommand{
				ID:     cmdpalette.CommandID(fmt.Sprintf("ext.notes.command.%03d", index)),
				Source: "ext.notes", Available: false,
			})
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "list", "--workspace", "workspace-1", "--source", "ext.notes",
			"--available=false", "-o", "jsonl",
		)
		if err != nil {
			t.Fatalf("cmd-palette list JSONL error = %v", err)
		}
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) != 512 {
			t.Fatalf("JSONL records = %d, want 512", len(lines))
		}
		for index, line := range lines {
			var command contract.CmdPaletteCommand
			if err := json.Unmarshal([]byte(line), &command); err != nil {
				t.Fatalf("json.Unmarshal(line %d) error = %v", index, err)
			}
			if command.Source != "ext.notes" || command.Available {
				t.Fatalf("line %d command = %#v", index, command)
			}
		}
	})

	t.Run("Should resolve ID name path and nested cwd to one workspace [IT-032]", func(t *testing.T) {
		client := newClient()
		client.DaemonClient = &stubClient{getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
			switch ref {
			case "workspace-acme", "acme", "/repo/acme", "/repo/acme/nested":
				return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
					ID: "workspace-acme", Name: "acme", RootDir: "/repo/acme",
				}}, nil
			default:
				return WorkspaceDetailRecord{}, errors.New("workspace not found")
			}
		}}
		for _, ref := range []string{"workspace-acme", "acme", "/repo/acme"} {
			if _, _, err := executeRootCommand(
				t, newTestDeps(t, client), "cmd-palette", "list", "--workspace", ref, "-o", "json",
			); err != nil {
				t.Fatalf("cmd-palette list --workspace %q error = %v", ref, err)
			}
			if client.listWorkspace != "workspace-acme" {
				t.Fatalf("resolved workspace for %q = %q, want workspace-acme", ref, client.listWorkspace)
			}
		}
		deps := newTestDeps(t, client)
		deps.getwd = func() (string, error) { return "/repo/acme/nested", nil }
		if _, _, err := executeRootCommand(t, deps, "cmd-palette", "list", "-o", "json"); err != nil {
			t.Fatalf("cmd-palette list from nested cwd error = %v", err)
		}
		if client.listWorkspace != "workspace-acme" {
			t.Fatalf("nested cwd resolved workspace = %q, want workspace-acme", client.listWorkspace)
		}
	})

	t.Run("Should preserve invocation results errors and exit codes [E2E-023]", func(t *testing.T) {
		t.Parallel()

		t.Run("Should return the inline tool result", func(t *testing.T) {
			t.Parallel()
			client := newClient()
			client.commands.Commands = []contract.CmdPaletteCommand{{
				ID:        "ext.notes.capture",
				Arguments: []cmdpalette.Argument{{Name: "title", Type: cmdpalette.ArgumentTypeText, Required: true}},
			}}
			client.invokeResult = contract.CmdPaletteInvokeResult{
				Status: cmdpalette.InvokeStatusOK, Result: json.RawMessage(`{"note_id":"note-a"}`),
			}
			stdout, _, err := executeRootCommand(
				t,
				newTestDeps(t, client),
				"cmd-palette", "invoke", "ext.notes.capture", "--workspace", "workspace-1",
				"--arg", "title=Standup follow-ups", "-o", "json",
			)
			if err != nil {
				t.Fatalf("cmd-palette invoke error = %v", err)
			}
			var output cmdPaletteInvokeOutputRecord
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("json.Unmarshal(output) error = %v", err)
			}
			var result map[string]string
			if err := json.Unmarshal(output.Result, &result); err != nil {
				t.Fatalf("json.Unmarshal(result) error = %v", err)
			}
			if output.Status != cmdpalette.InvokeStatusOK || result["note_id"] != "note-a" ||
				client.invokeRequest.Args["title"] != "Standup follow-ups" {
				t.Fatalf("invoke output/request = %#v / %#v", output, client.invokeRequest)
			}
		})

		t.Run("Should render missing arguments with exit two", func(t *testing.T) {
			t.Parallel()
			client := newClient()
			client.commands.Commands = []contract.CmdPaletteCommand{{
				ID:        "ext.notes.capture",
				Arguments: []cmdpalette.Argument{{Name: "title", Type: cmdpalette.ArgumentTypeText, Required: true}},
			}}
			client.invokeErr = &cmdPaletteAPIError{
				statusCode: http.StatusUnprocessableEntity,
				status:     "422 Unprocessable Entity",
				payload: contract.CmdPaletteError{
					Error: "invalid_arguments", Fields: map[string]string{"title": "required"},
				},
			}
			exitCode, _, stderr := executeRootCommandWithExit(
				t,
				newTestDeps(t, client),
				"cmd-palette", "invoke", "ext.notes.capture", "--workspace", "workspace-1",
			)
			if exitCode != 2 || !strings.Contains(stderr, `invalid arguments — missing required "title"`) {
				t.Fatalf("missing argument = exit %d stderr %q", exitCode, stderr)
			}
		})

		t.Run("Should return exit one for unknown commands and missing shells", func(t *testing.T) {
			t.Parallel()
			unknownClient := newClient()
			exitCode, _, _ := executeRootCommandWithExit(
				t,
				newTestDeps(t, unknownClient),
				"cmd-palette", "invoke", "ext.notes.missing", "--workspace", "workspace-1",
			)
			if exitCode != 1 {
				t.Fatalf("unknown command exit = %d, want 1", exitCode)
			}

			shellClient := newClient()
			shellClient.commands.Commands = []contract.CmdPaletteCommand{{ID: "window.close"}}
			shellClient.invokeErr = &cmdPaletteAPIError{
				statusCode: http.StatusPreconditionFailed,
				status:     "412 Precondition Failed",
				payload: contract.CmdPaletteError{
					Error:   "no_attached_shell",
					Message: "command changes UI state and needs an open Compozy shell",
				},
			}
			exitCode, _, stderr := executeRootCommandWithExit(
				t,
				newTestDeps(t, shellClient),
				"cmd-palette", "invoke", "window.close", "--workspace", "workspace-1",
			)
			if exitCode != 1 || !strings.Contains(stderr, "no attached shell client") {
				t.Fatalf("missing shell = exit %d stderr %q", exitCode, stderr)
			}
		})
	})
}

func TestCmdPaletteInvokeError(t *testing.T) {
	t.Parallel()

	err := errors.New("transport failed")
	var commandErr *commandExitError
	if !errors.As(cmdPaletteInvokeError(err), &commandErr) || commandErr.cliExitCode() != 1 {
		t.Fatal("transport invocation error must preserve exit code one")
	}
}
