package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/cmdpalette"
	"github.com/compozy/compozy/internal/session"
	toolspkg "github.com/compozy/compozy/internal/tools"
	workspacepkg "github.com/compozy/compozy/internal/workspace"
)

type nativeCmdPaletteRegistryStub struct {
	catalog       cmdpalette.Catalog
	invokeResult  cmdpalette.InvokeResult
	invokeRequest cmdpalette.InvokeRequest
}

func (s *nativeCmdPaletteRegistryStub) Catalog(
	context.Context,
	cmdpalette.WorkspaceID,
	cmdpalette.ClientID,
) (cmdpalette.Catalog, error) {
	return s.catalog, nil
}

func (s *nativeCmdPaletteRegistryStub) Clients(
	context.Context,
	cmdpalette.WorkspaceID,
) ([]cmdpalette.Client, error) {
	return nil, nil
}

func (s *nativeCmdPaletteRegistryStub) Invoke(
	_ context.Context,
	request cmdpalette.InvokeRequest,
) (cmdpalette.InvokeResult, error) {
	s.invokeRequest = request
	return s.invokeResult, nil
}

type nativeCmdPaletteWorkspaceStub struct {
	core.WorkspaceService
	resolved map[string]string
}

func (s *nativeCmdPaletteWorkspaceStub) Resolve(
	_ context.Context,
	ref string,
) (workspacepkg.ResolvedWorkspace, error) {
	id, ok := s.resolved[ref]
	if !ok {
		return workspacepkg.ResolvedWorkspace{}, errors.New("workspace not found")
	}
	return workspacepkg.ResolvedWorkspace{Workspace: workspacepkg.Workspace{ID: id}, WorkspaceID: id}, nil
}

type nativeCmdPaletteSessionStub struct {
	core.SessionManager
	info *session.Info
}

func (s *nativeCmdPaletteSessionStub) Status(context.Context, string) (*session.Info, error) {
	return s.info, nil
}

func TestNativeCmdPaletteTools(t *testing.T) {
	t.Parallel()

	t.Run("Should return the catalog command set with source filtering [UT-011]", func(t *testing.T) {
		t.Parallel()
		registry := &nativeCmdPaletteRegistryStub{catalog: cmdpalette.Catalog{Commands: []cmdpalette.ResolvedCommand{
			{Descriptor: cmdpalette.Descriptor{ID: "core.sessions.new", Source: cmdpalette.Source{Kind: cmdpalette.SourceKindCore}}},
			{Descriptor: cmdpalette.Descriptor{
				ID: "ext.notes.capture", Source: cmdpalette.Source{Kind: cmdpalette.SourceKindExtension, Extension: "notes"},
			}},
		}}}
		tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			CmdPalette: func() cmdpalette.Registry { return registry },
			Workspaces: &nativeCmdPaletteWorkspaceStub{resolved: map[string]string{"acme": "workspace-1"}},
		}}
		result, err := tools.cmdPaletteList(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDCmdPaletteList,
			Input:  json.RawMessage(`{"workspace":"acme","source":"ext.notes"}`),
		})
		if err != nil {
			t.Fatalf("cmdPaletteList() error = %v", err)
		}
		var payload struct {
			Commands []struct {
				ID string `json:"id"`
			} `json:"commands"`
		}
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(list result) error = %v", err)
		}
		if len(payload.Commands) != 1 || payload.Commands[0].ID != "ext.notes.capture" {
			t.Fatalf("commands = %#v, want ext.notes.capture", payload.Commands)
		}
	})

	t.Run("Should invoke as a control-plane caller and preserve approval pending", func(t *testing.T) {
		t.Parallel()
		registry := &nativeCmdPaletteRegistryStub{invokeResult: cmdpalette.InvokeResult{
			Status: cmdpalette.InvokeStatusApprovalPending, ApprovalID: "approval-1",
		}}
		tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			CmdPalette: func() cmdpalette.Registry { return registry },
			Workspaces: &nativeCmdPaletteWorkspaceStub{resolved: map[string]string{"acme": "workspace-1"}},
		}}
		result, err := tools.cmdPaletteInvoke(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDCmdPaletteInvoke,
			Input: json.RawMessage(
				`{"id":"ext.notes.purge","workspace":"acme","client":"client-1","args":{"force":true}}`,
			),
		})
		if err != nil {
			t.Fatalf("cmdPaletteInvoke() error = %v", err)
		}
		if registry.invokeRequest.Caller != cmdpalette.CallerControlPlane ||
			registry.invokeRequest.WorkspaceID != "workspace-1" ||
			registry.invokeRequest.ClientID != "client-1" {
			t.Fatalf("invoke request = %#v", registry.invokeRequest)
		}
		var payload struct {
			Status     string `json:"status"`
			ApprovalID string `json:"approval_id"`
		}
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(invoke result) error = %v", err)
		}
		if payload.Status != "approval_pending" || payload.ApprovalID != "approval-1" {
			t.Fatalf("invoke result = %#v", payload)
		}
	})

	t.Run("Should reject a foreign workspace for a session-bound caller [IT-032]", func(t *testing.T) {
		t.Parallel()
		tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			CmdPalette: func() cmdpalette.Registry { return &nativeCmdPaletteRegistryStub{} },
			Sessions:   &nativeCmdPaletteSessionStub{info: &session.Info{ID: "session-1", WorkspaceID: "workspace-a"}},
			Workspaces: &nativeCmdPaletteWorkspaceStub{resolved: map[string]string{"foreign": "workspace-b"}},
		}}
		_, err := tools.cmdPaletteList(t.Context(), toolspkg.Scope{SessionID: "session-1"}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDCmdPaletteList,
			Input:  json.RawMessage(`{"workspace":"foreign"}`),
		})
		var toolErr *toolspkg.ToolError
		if !errors.As(err, &toolErr) || toolErr.Code != toolspkg.ErrorCodeDenied {
			t.Fatalf("cmdPaletteList(foreign) error = %#v, want denied tool error", err)
		}
	})

	t.Run("Should honor the real tool approval gate before native dispatch [IT-021][E2E-024][E2E-026]", func(t *testing.T) {
		t.Parallel()
		completion := make(chan struct{})
		t.Cleanup(func() { close(completion) })
		coordinator := &recordingCmdPaletteApprovalCoordinator{ticket: toolspkg.ApprovalTicket{
			ApprovalID: "approval-native", InvocationID: "invocation-native", Completion: completion,
		}}
		toolRegistry := &recordingCmdPaletteToolRegistry{approvalRequired: true}
		executor := &cmdPaletteActionExecutor{
			tools: toolRegistry, approvals: coordinator, now: time.Now, approvalTTL: time.Minute,
		}
		descriptor := cmdpalette.Descriptor{
			ID: "notes.purge", Title: "Purge notes", Section: "Notes", Icon: "trash-2",
			Source:    cmdpalette.Source{Kind: cmdpalette.SourceKindCore},
			Action:    cmdpalette.Action{Kind: cmdpalette.ActionKindTool, Tool: "compozy__notes_purge"},
			Arguments: []cmdpalette.Argument{}, Destructive: true,
			Confirmation: &cmdpalette.Confirmation{Title: "Purge notes?", Confirm: "Purge"},
			Policy:       cmdpalette.ExecutionPolicy{SingleFlight: true},
		}
		registry, err := cmdpalette.NewRegistry(
			[]cmdpalette.ProviderRegistration{{
				Source:   descriptor.Source,
				Provider: nativeCmdPaletteStaticProvider{commands: []cmdpalette.Descriptor{descriptor}},
			}},
			nil,
			nil,
			executor,
			cmdpalette.WithInvocationIDGenerator(func() string { return "invocation-native" }),
		)
		if err != nil {
			t.Fatalf("NewRegistry() error = %v", err)
		}
		tools := &daemonNativeTools{deps: &daemonNativeToolsDeps{
			CmdPalette: func() cmdpalette.Registry { return registry },
			Workspaces: &nativeCmdPaletteWorkspaceStub{resolved: map[string]string{"acme": "workspace-1"}},
		}}
		result, err := tools.cmdPaletteInvoke(t.Context(), toolspkg.Scope{}, toolspkg.CallRequest{
			ToolID: toolspkg.ToolIDCmdPaletteInvoke,
			Input:  json.RawMessage(`{"id":"notes.purge","workspace":"acme"}`),
		})
		if err != nil {
			t.Fatalf("cmdPaletteInvoke() error = %v", err)
		}
		var payload contract.CmdPaletteInvokeResult
		if err := json.Unmarshal(result.Structured, &payload); err != nil {
			t.Fatalf("json.Unmarshal(result) error = %v", err)
		}
		if payload.Status != cmdpalette.InvokeStatusApprovalPending ||
			payload.ApprovalID != "approval-native" ||
			coordinator.request.Target.ToolID != "compozy__notes_purge" ||
			toolRegistry.call.ToolID != "" {
			t.Fatalf(
				"native approval = payload %#v request %#v premature call %#v",
				payload,
				coordinator.request,
				toolRegistry.call,
			)
		}
	})
}

type nativeCmdPaletteStaticProvider struct {
	commands []cmdpalette.Descriptor
}

func (p nativeCmdPaletteStaticProvider) ProvideCommands(
	context.Context,
	cmdpalette.WorkspaceID,
) ([]cmdpalette.Descriptor, error) {
	return append([]cmdpalette.Descriptor(nil), p.commands...), nil
}

func (p nativeCmdPaletteStaticProvider) StaticCommands() []cmdpalette.Descriptor {
	return append([]cmdpalette.Descriptor(nil), p.commands...)
}
