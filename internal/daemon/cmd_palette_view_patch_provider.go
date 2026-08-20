package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/cmdpalette"
	extensionpkg "github.com/compozy/compozy/internal/extension"
)

var _ extensionpkg.ViewPatchPublisher = (*extensionCmdPaletteProvider)(nil)

func (p *extensionCmdPaletteProvider) SubscribeViewPatches(
	ctx context.Context,
	request cmdpalette.ViewPatchSubscribeRequest,
) (<-chan cmdpalette.ViewPatchEvent, func(), error) {
	return p.SubscribeViewPatchesAfter(
		ctx, request.Workspace, request.ViewID, request.After, request.StreamEpoch,
	)
}

// SubscribeViewPatchesAfter replays retained patches with Sequence > after when epoch matches.
func (p *extensionCmdPaletteProvider) SubscribeViewPatchesAfter(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
	after int64,
	streamEpoch string,
) (<-chan cmdpalette.ViewPatchEvent, func(), error) {
	if _, err := p.requireDeclarativeView(ctx, workspaceID, viewID, ""); err != nil {
		return nil, nil, err
	}
	if p.patches == nil {
		return nil, nil, errors.New("daemon: extension palette view patch hub is unavailable")
	}
	return p.patches.subscribe(ctx, workspaceID, viewID, after, streamEpoch)
}

func (p *extensionCmdPaletteProvider) PublishViewPatch(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	extension string,
	patch cmdpalette.ViewPatch,
) error {
	if err := cmdpalette.ValidateViewPatch(patch); err != nil {
		return err
	}
	if _, err := p.requireDeclarativeView(ctx, workspaceID, patch.ViewID, extension); err != nil {
		return err
	}
	if p.patches == nil {
		return errors.New("daemon: extension palette view patch hub is unavailable")
	}
	if _, err := p.patches.publish(workspaceID, patch, ""); err != nil {
		return fmt.Errorf("daemon: publish extension palette view patch: %w", err)
	}
	return nil
}

// CloseViewPatches releases every live declarative view-patch subscriber.
func (p *extensionCmdPaletteProvider) CloseViewPatches() {
	if p == nil || p.patches == nil {
		return
	}
	p.patches.close()
}

func (p *extensionCmdPaletteProvider) requireDeclarativeView(
	ctx context.Context,
	workspaceID cmdpalette.WorkspaceID,
	viewID string,
	extension string,
) (extensionpkg.CmdPaletteProjectedView, error) {
	projection, err := p.projection(ctx, workspaceID)
	if err != nil {
		return extensionpkg.CmdPaletteProjectedView{}, err
	}
	viewID = strings.TrimSpace(viewID)
	var selected *extensionpkg.CmdPaletteProjectedView
	for index := range projection.Views {
		if projection.Views[index].ID == viewID {
			selected = &projection.Views[index]
			break
		}
	}
	if selected == nil {
		return extensionpkg.CmdPaletteProjectedView{}, &cmdpalette.ViewNotFoundError{ViewID: viewID}
	}
	if selected.UnavailableReason != "" {
		return extensionpkg.CmdPaletteProjectedView{}, fmt.Errorf(
			"daemon: extension palette view is unavailable: %s",
			selected.UnavailableReason,
		)
	}
	if selected.Program || selected.SourceTool == "" {
		return extensionpkg.CmdPaletteProjectedView{}, errors.New(
			"daemon: extension palette view is not declarative",
		)
	}
	if extension != "" && selected.Extension != extension {
		return extensionpkg.CmdPaletteProjectedView{}, fmt.Errorf(
			"daemon: extension %q does not own view %q",
			extension,
			viewID,
		)
	}
	return *selected, nil
}

func (p *extensionCmdPaletteProvider) paletteRuntime() extensionCmdPaletteRuntime {
	if p == nil {
		return nil
	}
	if p.palette != nil {
		return p.palette
	}
	if p.runtime == nil {
		return nil
	}
	runtime, ok := p.runtime().(extensionCmdPaletteRuntime)
	if !ok {
		return nil
	}
	return runtime
}
