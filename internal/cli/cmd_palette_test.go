package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/windowmanager"
)

type cmdPaletteTestClient struct {
	DaemonClient
	commands          contract.CmdPaletteCommandsResponse
	invokeResult      contract.CmdPaletteInvokeResult
	invokeErr         error
	invokeCommand     string
	invokeRequest     contract.CmdPaletteInvokeRequest
	listWorkspace     string
	listClient        string
	approvalStatus    contract.ToolApprovalStatusResponse
	canceledID        string
	personalization   contract.CmdPalettePersonalizationResponse
	resetResponse     contract.CmdPalettePersonalizationResetResponse
	resetWorkspace    string
	bindings          contract.SettingsWindowManagerResponse
	bindingsResult    contract.SettingsWindowManagerResponse
	bindingsUpdate    contract.UpdateSettingsWindowManagerRequest
	bindingsErr       error
	mutationWorkspace string
	pinCommand        string
	pinValue          bool
}

func (c *cmdPaletteTestClient) GetCmdPaletteBindings(
	_ context.Context,
	workspace string,
) (contract.SettingsWindowManagerResponse, error) {
	c.mutationWorkspace = workspace
	return c.bindings, nil
}

func (c *cmdPaletteTestClient) UpdateCmdPaletteBindings(
	_ context.Context,
	workspace string,
	request contract.UpdateSettingsWindowManagerRequest,
) (contract.SettingsWindowManagerResponse, error) {
	c.mutationWorkspace = workspace
	c.bindingsUpdate = request
	return c.bindingsResult, c.bindingsErr
}

func (c *cmdPaletteTestClient) SetCmdPalettePin(
	_ context.Context,
	workspace string,
	commandID string,
	pinned bool,
) (contract.CmdPalettePinResponse, error) {
	c.mutationWorkspace = workspace
	c.pinCommand = commandID
	c.pinValue = pinned
	return contract.CmdPalettePinResponse{Pinned: pinned}, nil
}

func (c *cmdPaletteTestClient) GetCmdPalettePersonalization(
	_ context.Context,
	workspace string,
) (contract.CmdPalettePersonalizationResponse, error) {
	c.personalization.Workspace = cmdpalette.WorkspaceID(workspace)
	return c.personalization, nil
}

func (c *cmdPaletteTestClient) ResetCmdPalettePersonalization(
	_ context.Context,
	workspace string,
) (contract.CmdPalettePersonalizationResetResponse, error) {
	c.resetWorkspace = workspace
	return c.resetResponse, nil
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
		client.commands.Commands = []contract.CmdPaletteCommand{
			{
				ID:        "core.tools.run",
				Arguments: []cmdpalette.Argument{{Name: "force", Type: cmdpalette.ArgumentTypeCheckbox}},
			},
		}
		client.invokeResult = contract.CmdPaletteInvokeResult{
			Status: cmdpalette.InvokeStatusApprovalPending, ApprovalID: "approval-1",
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette",
			"invoke",
			"core.tools.run",
			"--workspace",
			"workspace-1",
			"--arg",
			"force=true",
			"-o",
			"json",
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
		client.invokeErr = &daemonAPIError{
			statusCode: http.StatusUnprocessableEntity,
			status:     "422 Unprocessable Entity",
		}
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
		client.commands.Commands = []contract.CmdPaletteCommand{
			{
				ID:        "core.tools.run",
				Arguments: []cmdpalette.Argument{{Name: "force", Type: cmdpalette.ArgumentTypeCheckbox}},
			},
		}
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

	t.Run("Should show and reset workspace personalization", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.personalization = contract.CmdPalettePersonalizationResponse{
			Pins: []cmdpalette.CommandID{"session.new"}, Recents: 18,
			FrecencyEntries: 64, QueryAssociations: 21,
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "personalization", "show", "--workspace", "workspace-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette personalization show error = %v", err)
		}
		var summary contract.CmdPalettePersonalizationResponse
		if err := json.Unmarshal([]byte(stdout), &summary); err != nil {
			t.Fatalf("json.Unmarshal(personalization) error = %v", err)
		}
		if summary.Workspace != "workspace-1" || len(summary.Pins) != 1 ||
			summary.FrecencyEntries != 64 || summary.QueryAssociations != 21 {
			t.Fatalf("personalization summary = %#v", summary)
		}

		client.resetResponse.Status = "reset"
		stdout, _, err = executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "personalization", "reset", "--workspace", "workspace-1",
		)
		if err != nil {
			t.Fatalf("cmd-palette personalization reset error = %v", err)
		}
		want := "Reset palette personalization for workspace workspace-1 (pins, recents, frecency, query learning)."
		if strings.TrimSpace(stdout) != want || client.resetWorkspace != "workspace-1" {
			t.Fatalf("reset output/workspace = %q/%q, want %q/workspace-1", stdout, client.resetWorkspace, want)
		}
	})

	t.Run("Should bind with explicit overwrite and name the unbound owner", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.bindings = contract.SettingsWindowManagerResponse{
			Config: contract.SettingsWindowManagerConfigPayload{
				Shortcuts: map[string]windowmanager.ShortcutBinding{},
			},
			EffectiveShortcuts: map[string]windowmanager.ShortcutBinding{
				"session.new": {"meta+shift+KeyN"},
			},
		}
		client.bindingsResult = contract.SettingsWindowManagerResponse{
			EffectiveShortcuts: map[string]windowmanager.ShortcutBinding{
				"ext.notes.capture": {"meta+shift+KeyN"},
			},
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "bind", "ext.notes.capture", "meta+shift+KeyN",
			"--overwrite", "--workspace", "workspace-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette bind error = %v", err)
		}
		var result cmdPaletteBindingMutationResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(bind output) error = %v", err)
		}
		if result.Status != "ok" || result.UnboundOwner != "session.new" ||
			!reflect.DeepEqual(result.Bound, []string{"meta+shift+KeyN"}) {
			t.Fatalf("bind output = %#v, want transferred session.new binding", result)
		}
		if !client.bindingsUpdate.Overwrite || client.bindingsUpdate.Shortcuts == nil ||
			!reflect.DeepEqual(
				(*client.bindingsUpdate.Shortcuts)["ext.notes.capture"],
				windowmanager.ShortcutBinding{"meta+shift+KeyN"},
			) {
			t.Fatalf("bind request = %#v, want explicit overwrite", client.bindingsUpdate)
		}
	})

	t.Run("Should bind and unbind a desktop-global hotkey through whole-map patches", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.bindings.Config.GlobalShortcuts = map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: windowmanager.DefaultGlobalSummonChord,
		}
		client.bindingsResult.Config.GlobalShortcuts = map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: windowmanager.DefaultGlobalSummonChord,
			"session.new": "meta+shift+KeyN",
		}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "bind", "session.new", "meta+shift+KeyN",
			"--global", "--workspace", "workspace-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette bind --global error = %v", err)
		}
		var result cmdPaletteBindingMutationResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(global bind output) error = %v", err)
		}
		if !reflect.DeepEqual(result.Bound, []string{"meta+shift+KeyN"}) ||
			client.bindingsUpdate.GlobalShortcuts == nil ||
			len(*client.bindingsUpdate.GlobalShortcuts) != 2 {
			t.Fatalf("global bind output/request = %#v / %#v", result, client.bindingsUpdate)
		}

		client.bindings.Config.GlobalShortcuts = *client.bindingsUpdate.GlobalShortcuts
		client.bindingsUpdate = contract.UpdateSettingsWindowManagerRequest{}
		stdout, _, err = executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "unbind", "session.new", "--global",
			"--workspace", "workspace-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette unbind --global error = %v", err)
		}
		var unbound cmdPaletteStatusResult
		if err := json.Unmarshal([]byte(stdout), &unbound); err != nil {
			t.Fatalf("json.Unmarshal(global unbind output) error = %v", err)
		}
		if unbound.Status != "ok" || client.bindingsUpdate.GlobalShortcuts == nil {
			t.Fatalf("global unbind output/request = %q / %#v", stdout, client.bindingsUpdate)
		}
		if _, exists := (*client.bindingsUpdate.GlobalShortcuts)["session.new"]; exists {
			t.Fatal("global unbind request retained session.new")
		}
	})

	t.Run("Should preserve the structured shortcut conflict and exit one", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.bindings.Config.Shortcuts = map[string]windowmanager.ShortcutBinding{}
		client.bindingsErr = &cmdPaletteMutationAPIError{
			statusCode: http.StatusConflict,
			payload: contract.SettingsWindowManagerMutationError{
				Error: "shortcut_conflict", Owner: "session.new", Chord: "meta+shift+KeyN",
			},
		}

		exitCode, _, stderr := executeRootCommandWithExit(
			t,
			newTestDeps(t, client),
			"cmd-palette", "bind", "ext.notes.capture", "meta+shift+KeyN", "--workspace", "workspace-1",
		)
		if exitCode != 1 || !strings.Contains(
			stderr,
			`shortcut conflict — meta+shift+KeyN is used by "session.new". Re-run with --overwrite to take it.`,
		) {
			t.Fatalf("bind conflict = exit %d stderr %q", exitCode, stderr)
		}
	})

	t.Run("Should set and clear aliases through the settings patch", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.bindings.Aliases = map[string]string{"session.new": "new"}

		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "alias", "set", "ext.notes.capture", "cap",
			"--workspace", "workspace-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette alias set error = %v", err)
		}
		var result cmdPaletteAliasMutationResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(alias output) error = %v", err)
		}
		if result.Status != "ok" || result.Alias != "cap" || client.bindingsUpdate.Aliases == nil ||
			(*client.bindingsUpdate.Aliases)["ext.notes.capture"] != "cap" ||
			(*client.bindingsUpdate.Aliases)["session.new"] != "new" {
			t.Fatalf("alias result/request = %#v / %#v", result, client.bindingsUpdate)
		}

		client.bindings.Aliases = map[string]string{"ext.notes.capture": "cap"}
		stdout, _, err = executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "alias", "clear", "ext.notes.capture", "--workspace", "workspace-1",
		)
		if err != nil {
			t.Fatalf("cmd-palette alias clear error = %v", err)
		}
		if strings.TrimSpace(stdout) != `{"status":"ok"}` || client.bindingsUpdate.Aliases == nil ||
			len(*client.bindingsUpdate.Aliases) != 0 {
			t.Fatalf("alias clear output/request = %q / %#v", stdout, client.bindingsUpdate)
		}
	})

	t.Run("Should list binding truth and mutate pins", func(t *testing.T) {
		t.Parallel()
		client := newClient()
		client.bindings = contract.SettingsWindowManagerResponse{
			EffectiveShortcuts: map[string]windowmanager.ShortcutBinding{
				"ext.notes.capture": {"alt+shift+KeyN"},
			},
			Aliases: map[string]string{"ext.notes.capture": "cap"},
			ExtensionDefaults: []contract.SettingsWindowManagerDefaultPayload{{
				CommandID: "ext.other.jump", Binding: windowmanager.ShortcutBinding{"meta+KeyJ"},
				Dormant: true, ConflictWith: "window.tab.jump.1",
			}},
		}
		stdout, _, err := executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "bindings", "--workspace", "workspace-1", "-o", "json",
		)
		if err != nil {
			t.Fatalf("cmd-palette bindings error = %v", err)
		}
		var result cmdPaletteBindingsResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("json.Unmarshal(bindings output) error = %v", err)
		}
		if result.Aliases["ext.notes.capture"] != "cap" || len(result.DormantDefaults) != 1 ||
			result.Conflicts == nil {
			t.Fatalf("bindings output = %#v, want aliases, dormant defaults, and empty conflicts", result)
		}

		stdout, _, err = executeRootCommand(
			t,
			newTestDeps(t, client),
			"cmd-palette", "pin", "session.new", "--workspace", "workspace-1",
		)
		if err != nil {
			t.Fatalf("cmd-palette pin error = %v", err)
		}
		if strings.TrimSpace(stdout) != `{"status":"ok","pinned":true}` ||
			client.pinCommand != "session.new" || !client.pinValue {
			t.Fatalf("pin output/request = %q / %q %t", stdout, client.pinCommand, client.pinValue)
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
		client.DaemonClient = &stubClient{
			getWorkspaceFn: func(_ context.Context, ref string) (WorkspaceDetailRecord, error) {
				switch ref {
				case "workspace-acme", "acme", "/repo/acme", "/repo/acme/nested":
					return WorkspaceDetailRecord{Workspace: WorkspaceRecord{
						ID: "workspace-acme", Name: "acme", RootDir: "/repo/acme",
					}}, nil
				default:
					return WorkspaceDetailRecord{}, errors.New("workspace not found")
				}
			},
		}
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
					Message: "command changes UI state and needs an open CompozyOS shell",
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
