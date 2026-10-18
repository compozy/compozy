package windowmanager

import (
	"fmt"
	"iter"
	"math"
	"strings"
)

func (r *reducer) openWindow(snapshot *Snapshot, command OpenWindowCommand) (bool, error) {
	if command.RestoreWindowID != nil {
		return r.restoreWindow(snapshot, *command.RestoreWindowID)
	}
	window, err := r.newOpenWindow(snapshot, command.Window)
	if err != nil {
		return false, err
	}
	windowID := window.ID
	desktopID := window.DesktopID
	snapshot.Windows[windowID] = window
	if stackTarget := r.openStackTarget(snapshot, command.Window, window); stackTarget != nil {
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
		changed, groupErr := r.groupWindows(snapshot, GroupWindowsCommand{
			TargetWindowID: *stackTarget,
			WindowIDs:      []WindowID{windowID},
		})
		if groupErr != nil {
			return false, groupErr
		}
		return changed, nil
	}
	// The tab policy falls back to beside-focus placement when there is no
	// frame to join; an explicit floating open never tiles.
	insertTiled := command.Window.InsertTiled ||
		(!command.Window.Floating && r.config.NewWindowPolicy != NewWindowFloating)
	if insertTiled {
		tiled, err := r.placeTiledOpen(snapshot, window, command.Window.BesideWindowID)
		if err != nil {
			delete(snapshot.Windows, windowID)
			return false, err
		}
		insertTiled = tiled
	}
	if !insertTiled {
		desktopIndex, _ := desktopIndexByID(snapshot, desktopID)
		window.FloatingRect = cascadeOpenRect(snapshot.Desktops[desktopIndex], snapshot.Windows, window.FloatingRect)
		snapshot.Windows[windowID] = window
		snapshot.Desktops[desktopIndex].Floating = append(snapshot.Desktops[desktopIndex].Floating, windowID)
	}
	r.revealPlacedWindow(snapshot, windowID)
	r.changes.window(windowID)
	r.changes.desktop(desktopID)
	return true, nil
}

// openStackTarget is the window whose frame a new window joins as a tab: the
// spec's explicit stack target, else — under the tab policy, for an open that
// asks for no other placement — the client's focused window when it is placed
// and visible on the open's desktop. A clientless open has no focus, so the
// policy never folds it into a frame the user is looking at; like a focus on
// another desktop or a minimized focus, it falls back to beside-focus placement.
func (r *reducer) openStackTarget(snapshot *Snapshot, spec WindowSpec, window Window) *WindowID {
	if spec.StackTargetWindowID != nil {
		return spec.StackTargetWindowID
	}
	if r.config.NewWindowPolicy != NewWindowTab || spec.Floating || spec.InsertTiled ||
		spec.BesideWindowID != nil || r.focusedWindow == nil {
		return nil
	}
	focused, exists := snapshot.Windows[*r.focusedWindow]
	if !exists || focused.Minimized || focused.DesktopID != window.DesktopID {
		return nil
	}
	if _, placed := findWindowPlacement(snapshot, focused.ID); !placed {
		return nil
	}
	return &focused.ID
}

// placeTiledOpen tiles a new window beside its anchor: the spec's explicit
// BesideWindowID, else the client's focused window. An anchor that floats —
// alone or as a tab of a floating frame — keeps the new window floating; an
// anchor that is missing, unplaced (minimized) or on another desktop is
// no anchor at all, so an empty desktop still gets one full-frame pane instead
// of a stale focus pointer turning the open into a floating window. With no
// anchor on a desktop that already has tiles, the window floats. It reports
// whether the window was tiled.
func (r *reducer) placeTiledOpen(snapshot *Snapshot, window Window, besideID *WindowID) (bool, error) {
	anchorID := besideID
	if anchorID == nil {
		anchorID = r.focusedWindow
	}
	if anchorID != nil {
		anchor, exists := snapshot.Windows[*anchorID]
		placement, placed := findWindowPlacement(snapshot, *anchorID)
		if exists && placed && anchor.DesktopID == window.DesktopID {
			// Classify by the containing topology, not the window's own
			// placement: a tab of a floating frame is "stacked" but floats.
			if placement.groupIndex < 0 {
				return false, nil
			}
			if err := insertRelative(snapshot, anchor.ID, window.ID, DropAfter, r.generate); err != nil {
				return false, err
			}
			r.changes.window(anchor.ID)
			r.revealPlacedWindow(snapshot, window.ID)
			return true, nil
		}
	}
	desktopIndex, exists := desktopIndexByID(snapshot, window.DesktopID)
	if !exists {
		return false, fmt.Errorf("desktop %q: %w", window.DesktopID, ErrDesktopNotFound)
	}
	if len(snapshot.Desktops[desktopIndex].Groups) > 0 {
		return false, nil
	}
	leaf, err := newLeaf(window.ID, r.generate)
	if err != nil {
		return false, err
	}
	groupID, err := r.generate("group")
	if err != nil {
		return false, fmt.Errorf("generate group ID: %w", err)
	}
	snapshot.Desktops[desktopIndex].Groups = append(
		snapshot.Desktops[desktopIndex].Groups,
		LayoutGroup{ID: GroupID(groupID), Frame: fullRect(), Root: leaf},
	)
	r.changes.group(GroupID(groupID))
	r.changes.node(leaf.ID)
	return true, nil
}

// cascadeOpenRect places a new floating window on the first cascade candidate
// that no visible floating window of the same desktop already occupies, so an
// open never lands exactly over another window. Candidates are distinct, so at
// most len(Floating) of them can be taken: the walk stops after one more, and
// keeps the requested rect only when the desktop has no candidate left.
func cascadeOpenRect(desktop Desktop, windows map[WindowID]Window, rect NormalizedRect) NormalizedRect {
	occupied := func(candidate NormalizedRect) bool {
		for _, id := range desktop.Floating {
			other, exists := windows[id]
			if exists && !other.Minimized && sameRect(other.FloatingRect, candidate) {
				return true
			}
		}
		return false
	}
	remaining := len(desktop.Floating) + 1
	for candidate := range cascadeCandidates(rect) {
		if !occupied(candidate) {
			return candidate
		}
		if remaining--; remaining == 0 {
			break
		}
	}
	return rect
}

// cascadeCandidates yields the rects a floating open may take: the requested
// rect, then cascadeStep moves down-right along its diagonal. A diagonal ends
// before either edge would clamp it, and the walk restarts one step right of
// the requested origin — then, once columns run out, one step below it. Each
// diagonal starts at a different offset from the origin, so no rect repeats
// and the sequence is finite.
func cascadeCandidates(rect NormalizedRect) iter.Seq[NormalizedRect] {
	return func(yield func(NormalizedRect) bool) {
		const epsilon = 1e-9
		fits := func(x, y float64) bool {
			return x+rect.Width <= 1+epsilon && y+rect.Height <= 1+epsilon
		}
		diagonal := func(x, y float64) bool {
			for step := 0.0; fits(x+step*cascadeStep, y+step*cascadeStep); step++ {
				candidate := rect
				candidate.X, candidate.Y = x+step*cascadeStep, y+step*cascadeStep
				if !yield(candidate) {
					return false
				}
			}
			return true
		}
		for column := 0.0; fits(rect.X+column*cascadeStep, rect.Y); column++ {
			if !diagonal(rect.X+column*cascadeStep, rect.Y) {
				return
			}
		}
		for row := 1.0; fits(rect.X, rect.Y+row*cascadeStep); row++ {
			if !diagonal(rect.X, rect.Y+row*cascadeStep) {
				return
			}
		}
	}
}

func sameRect(left, right NormalizedRect) bool {
	const epsilon = 1e-9
	return math.Abs(left.X-right.X) < epsilon && math.Abs(left.Y-right.Y) < epsilon &&
		math.Abs(left.Width-right.Width) < epsilon && math.Abs(left.Height-right.Height) < epsilon
}

func (r *reducer) newOpenWindow(snapshot *Snapshot, spec WindowSpec) (Window, error) {
	windowID := spec.ID
	if windowID == "" {
		for range 128 {
			generated, err := r.generate("window")
			if err != nil {
				return Window{}, fmt.Errorf("generate window ID: %w", err)
			}
			candidate := WindowID(generated)
			if !reservedWindowID(snapshot, candidate) {
				windowID = candidate
				break
			}
		}
		if windowID == "" {
			return Window{}, fmt.Errorf("generate unique window ID: %w", ErrInvalidCommand)
		}
	}
	if reservedWindowID(snapshot, windowID) {
		return Window{}, fmt.Errorf("window %q already exists: %w", windowID, ErrInvalidCommand)
	}
	if spec.Floating && (spec.InsertTiled || spec.StackTargetWindowID != nil || spec.BesideWindowID != nil) {
		return Window{}, fmt.Errorf("floating open cannot also tile or join a stack: %w", ErrInvalidCommand)
	}
	if spec.StackTargetWindowID != nil {
		target, exists := snapshot.Windows[*spec.StackTargetWindowID]
		if !exists {
			return Window{}, fmt.Errorf("stack target window %q: %w", *spec.StackTargetWindowID, ErrWindowNotFound)
		}
		if spec.DesktopID != "" && spec.DesktopID != target.DesktopID {
			return Window{}, fmt.Errorf("stack target belongs to another desktop: %w", ErrInvalidCommand)
		}
		spec.DesktopID = target.DesktopID
	}
	app := strings.TrimSpace(spec.App)
	if app == "" {
		return Window{}, fmt.Errorf("window app is required: %w", ErrInvalidCommand)
	}
	desktopID, err := r.resolveOpenDesktop(snapshot, spec.DesktopID)
	if err != nil {
		return Window{}, err
	}
	route, err := CanonicalRouteIntent(spec.Route)
	if err != nil {
		return Window{}, fmt.Errorf("window route: %w: %w", err, ErrInvalidCommand)
	}
	return Window{
		ID:           windowID,
		App:          app,
		InstanceKey:  clonePointer(spec.InstanceKey),
		Route:        route,
		DesktopID:    desktopID,
		Placement:    WindowPlacementFloating,
		FloatingRect: clampRect(spec.FloatingRect),
	}, nil
}

func (r *reducer) resolveOpenDesktop(snapshot *Snapshot, requested DesktopID) (DesktopID, error) {
	if requested != "" {
		if _, exists := desktopIndexByID(snapshot, requested); !exists {
			return "", fmt.Errorf("desktop %q: %w", requested, ErrDesktopNotFound)
		}
		return requested, nil
	}
	if r.focusedWindow != nil {
		if focused, exists := snapshot.Windows[*r.focusedWindow]; exists {
			return focused.DesktopID, nil
		}
	}
	if len(snapshot.Desktops) == 0 {
		return "", ErrFinalDesktop
	}
	return snapshot.Desktops[0].ID, nil
}
