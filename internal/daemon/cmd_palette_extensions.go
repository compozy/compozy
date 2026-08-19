package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type extensionCmdPaletteRuntime interface {
	CmdPalette(string) (extensionpkg.CmdPaletteProjection, error)
}

type extensionCmdPaletteProvider struct {
	runtime func() extensionRuntime
	tools   toolspkg.Registry
}

var (
	_ cmdpalette.ContributionProvider = (*extensionCmdPaletteProvider)(nil)
	_ cmdpalette.DynamicViewProvider  = (*extensionCmdPaletteProvider)(nil)
)

func (p *extensionCmdPaletteProvider) ProvideCommands(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) ([]cmdpalette.Descriptor, error) {
	contribution, err := p.ProvideContribution(ctx, workspaceID)
	return contribution.Commands, err
}

func (p *extensionCmdPaletteProvider) ProvideContribution(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) (cmdpalette.Contribution, error) {
	projection, err := p.projection(ctx, workspaceID)
	if err != nil {
		return cmdpalette.Contribution{}, err
	}
	result := cmdpalette.Contribution{
		Commands: make([]cmdpalette.Descriptor, 0, len(projection.Commands)),
		Sources:  make([]cmdpalette.SourceStatus, 0, len(projection.Sources)),
		Defaults: make([]cmdpalette.ExtensionDefaultShortcut, 0, len(projection.Defaults)),
	}
	for _, command := range projection.Commands {
		result.Commands = append(result.Commands, extensionCmdPaletteDescriptor(command))
	}
	for _, source := range projection.Sources {
		result.Sources = append(result.Sources, cmdpalette.SourceStatus{
			Source: source.Source, Status: cmdpalette.SourceHealth(source.Status), Reason: source.Reason,
		})
	}
	for _, shortcut := range projection.Defaults {
		result.Defaults = append(result.Defaults, cmdpalette.ExtensionDefaultShortcut{
			CommandID: cmdpalette.CommandID(shortcut.CommandID), Chord: shortcut.Chord,
			Source: "ext." + shortcut.Extension, Active: shortcut.Active,
		})
	}
	return result, nil
}

func (p *extensionCmdPaletteProvider) ProvideViews(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) ([]cmdpalette.ViewDescriptor, error) {
	projection, err := p.projection(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	views := make([]cmdpalette.ViewDescriptor, 0, len(projection.Views))
	for _, view := range projection.Views {
		descriptor := cmdpalette.ViewDescriptor{
			ID: view.ID, Title: view.Title, Kind: cmdpalette.ViewKind(view.Kind),
			Program: view.Program, Extension: view.Extension,
		}
		if view.SourceTool != "" {
			descriptor.Source = &cmdpalette.ViewToolSource{Tool: view.SourceTool, ReadOnly: true}
		}
		views = append(views, descriptor)
	}
	return views, nil
}

func (p *extensionCmdPaletteProvider) OpenSource(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
) (cmdpalette.ViewPayload, error) {
	if p == nil || p.tools == nil {
		return cmdpalette.ViewPayload{}, errors.New("daemon: extension palette tool registry is unavailable")
	}
	projection, err := p.projection(ctx, workspaceID)
	if err != nil {
		return cmdpalette.ViewPayload{}, err
	}
	var selected *extensionpkg.CmdPaletteProjectedView
	for index := range projection.Views {
		if projection.Views[index].ID == strings.TrimSpace(viewID) {
			selected = &projection.Views[index]
			break
		}
	}
	if selected == nil {
		return cmdpalette.ViewPayload{}, &cmdpalette.ViewNotFoundError{ViewID: viewID}
	}
	if selected.UnavailableReason != "" {
		return cmdpalette.ViewPayload{}, fmt.Errorf(
			"daemon: extension palette view is unavailable: %s",
			selected.UnavailableReason,
		)
	}
	if selected.Program || selected.SourceTool == "" {
		return cmdpalette.ViewPayload{}, errors.New("daemon: extension palette view is not declarative")
	}
	callID := "cmd-palette-view:" + uuid.NewString()
	result, err := p.tools.Call(ctx, toolspkg.Scope{
		WorkspaceID: string(workspaceID), SessionID: callID, ActorKind: "cmd_palette", Operator: true,
	}, toolspkg.CallRequest{
		ToolID: toolspkg.ToolID(selected.SourceTool), ToolCallID: callID, SessionID: callID,
		WorkspaceID: string(workspaceID), CorrelationID: callID, Input: json.RawMessage(`{}`),
	})
	if err != nil {
		return cmdpalette.ViewPayload{}, fmt.Errorf("daemon: invoke extension palette view source: %w", err)
	}
	payload := result.Structured
	if len(payload) == 0 && len(result.Content) == 1 {
		payload = result.Content[0].Data
	}
	if len(payload) == 0 {
		return cmdpalette.ViewPayload{}, errors.New(
			"daemon: extension palette view source returned no structured payload",
		)
	}
	var view cmdpalette.ViewPayload
	if err := json.Unmarshal(payload, &view); err != nil {
		return cmdpalette.ViewPayload{}, fmt.Errorf("daemon: decode extension palette view source: %w", err)
	}
	return view, nil
}

func (p *extensionCmdPaletteProvider) projection(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
) (extensionpkg.CmdPaletteProjection, error) {
	if ctx == nil {
		return extensionpkg.CmdPaletteProjection{}, errors.New("daemon: extension palette context is required")
	}
	if err := ctx.Err(); err != nil {
		return extensionpkg.CmdPaletteProjection{}, err
	}
	if p == nil || p.runtime == nil {
		return extensionpkg.CmdPaletteProjection{}, nil
	}
	runtime, ok := p.runtime().(extensionCmdPaletteRuntime)
	if !ok || runtime == nil {
		return extensionpkg.CmdPaletteProjection{}, nil
	}
	projection, err := runtime.CmdPalette(string(workspaceID))
	if err != nil {
		return extensionpkg.CmdPaletteProjection{}, fmt.Errorf("daemon: project extension command palette: %w", err)
	}
	return projection, nil
}

func extensionCmdPaletteDescriptor(command extensionpkg.CmdPaletteProjectedCommand) cmdpalette.Descriptor {
	arguments := make([]cmdpalette.Argument, 0, len(command.Arguments))
	for _, argument := range command.Arguments {
		arguments = append(arguments, cmdpalette.Argument{
			Name: argument.Name, Type: cmdpalette.ArgumentType(argument.Type),
			Placeholder: argument.Placeholder, Required: argument.Required,
			Options: append([]string(nil), argument.Options...),
		})
	}
	var confirmation *cmdpalette.Confirmation
	if command.Confirmation != nil {
		confirmation = &cmdpalette.Confirmation{
			Title: command.Confirmation.Title, Body: command.Confirmation.Body,
			Confirm: command.Confirmation.Confirm,
		}
	}
	return cmdpalette.Descriptor{
		ID: cmdpalette.CommandID(command.ID), Title: command.Title, Section: command.Section,
		Icon: command.Icon, Keywords: append([]string(nil), command.Keywords...),
		Source: cmdpalette.Source{Kind: cmdpalette.SourceKindExtension, Extension: command.Extension},
		Action: cmdpalette.Action{
			Kind: cmdpalette.ActionKind(command.Action.Kind), Tool: command.Action.Tool,
			View: command.Action.View, App: command.Action.App, URL: command.Action.URL,
			Args: command.Action.Args,
		},
		Arguments: arguments, Destructive: command.Destructive, Confirmation: confirmation,
		Policy: cmdpalette.ExecutionPolicy{
			SingleFlight: command.Execution.SingleFlight, RetrySafe: command.Execution.RetrySafe,
		},
		ProviderUnavailableReason: command.UnavailableReason,
	}
}
