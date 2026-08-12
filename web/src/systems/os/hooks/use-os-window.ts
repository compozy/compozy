import { useEffect, useLayoutEffect, type FocusEvent, type PointerEvent } from "react";
import type { Rnd, RndDragCallback, RndResizeCallback, RndResizeStartCallback } from "react-rnd";
import { useSelector, useStore } from "@xstate/store-react";

import type { OsTrafficLightAction } from "../components/os-traffic-lights";
import { bindLayoutGestureCancellation } from "../lib/layout-gesture-cancellation";
import { clampFloatingRect } from "../lib/layout-projection";
import type { OsWindowFrameModel } from "../lib/group-projection";
import { resolveSnapTarget, type OccupiedSnapCandidate } from "../lib/snap-targets";
import type { OsRect, WindowManagerCommandOutcome } from "../lib/os-types";
import { snapTargetConfigFromConfig } from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";
import { finishWindowManagerGesture } from "../stores/window-manager-store-commands";
import { useDesktop } from "./use-desktop";
import { useAnimationFrameLatest } from "./use-animation-frame-latest";
import { useOsShell } from "./use-os-shell";
import { useOsWindowResize } from "./use-os-window-resize";
import { osWindowInteractionLogic } from "./os-window-interaction-store";
import {
  useWindowManagerGestureActive,
  useWindowManagerWorkArea,
} from "./use-window-manager-store";

type OsDragEvent = Parameters<RndDragCallback>[0];

interface PendingDragPreview {
  point: { x: number; y: number };
  swapModifierActive: boolean;
}

export const OS_WINDOW_DRAG_HANDLE_CLASS = "os-window-drag-handle";
export const OS_WINDOW_DRAG_CANCEL_SELECTOR = [
  '[data-slot="os-traffic-lights"]',
  '[data-slot="os-window-tab"]',
  '[data-slot="os-window-tab-add"]',
  '[data-slot="topbar-back"]',
  '[data-slot="topbar-crumb"]',
  '[data-slot="topbar-crumb-more"]',
  '[data-slot="topbar-trailing"]',
].join(", ");

export interface OsWindowModel {
  focused: boolean;
  keepMounted: boolean;
  rect: OsRect;
  registerRnd: (instance: Rnd | null) => void;
  /** Live pixel ceiling for the active tiled edge drag; null when unconstrained. */
  resizeMax: { width: number; height: number } | null;
  handleTrafficLight: (action: OsTrafficLightAction) => void;
  handlePointerEnter: () => void;
  handlePointerDownCapture: (event: PointerEvent<HTMLElement>) => void;
  handleFocusCapture: (event: FocusEvent<HTMLElement>) => void;
  handleDragStart: RndDragCallback;
  handleDrag: RndDragCallback;
  handleDragStop: RndDragCallback;
  handleResizeStart: RndResizeStartCallback;
  handleResizeStop: RndResizeCallback;
}

function dragPoint(event: OsDragEvent): { x: number; y: number } | null {
  const source = (event as { nativeEvent?: Event }).nativeEvent ?? (event as Event);
  if (source instanceof MouseEvent) return { x: source.clientX, y: source.clientY };
  if (typeof TouchEvent !== "undefined" && source instanceof TouchEvent) {
    const touch = source.touches[0];
    return touch ? { x: touch.clientX, y: touch.clientY } : null;
  }
  return null;
}

/**
 * Snap resolution, window rects, and rnd positions all live in layer
 * coordinates; pointer events arrive in viewport coordinates. Subtracting the
 * measured layer origin keeps every hot zone aligned with what the user sees.
 */
function layerPoint(event: OsDragEvent): { x: number; y: number } | null {
  const client = dragPoint(event);
  if (client === null) return null;
  const origin = windowManagerStore.getSnapshot().context.workArea?.origin;
  if (origin === undefined) return null;
  return { x: client.x - origin.x, y: client.y - origin.y };
}

function modifierActive(
  event: OsDragEvent,
  modifier: "alt" | "control" | "meta" | "shift" | "none"
): boolean {
  switch (modifier) {
    case "alt":
      return "altKey" in event && event.altKey;
    case "control":
      return "ctrlKey" in event && event.ctrlKey;
    case "meta":
      return "metaKey" in event && event.metaKey;
    case "shift":
      return "shiftKey" in event && event.shiftKey;
    case "none":
      return false;
  }
}

/**
 * Semantic frame gestures; pixels remain preview-only until one command
 * commits. The gesture source is the frame's active member: a real tab frame
 * (floating or tiled) moves as one unit (`move_group`) because the deck bar is
 * its only drag handle — the browser contract where the strip drags the whole
 * window and only a tab drags alone. Solo windows and adaptive viewport
 * stacks keep the established drag-away/snap semantics. Group moves resolve
 * the same zones as solo windows — tiles arrange a tiled stack, sides splice
 * the frame in as one node, occupied centers swap whole units, and zoom lifts
 * the whole frame.
 */
export function useOsWindow(frame: OsWindowFrameModel): OsWindowModel {
  const { manager, coordinator } = useOsShell();
  const activeId = frame.activeWindowId;
  const memberIds = new Set(frame.members);
  const gestureActive = useWindowManagerGestureActive(activeId);
  const workArea = useWindowManagerWorkArea();
  const activeDesktopId = useDesktop(state => state.activeDesktopId);
  const focused = useDesktop(state => state.focusedId !== null && memberIds.has(state.focusedId));
  // Adaptive viewport stacks (stackId null) are squeezed panes, not tab
  // groups — their members still drag away individually.
  const movesAsUnit = frame.stackId !== null && frame.members.length > 1;
  const visibleNow = frame.desktopId === activeDesktopId && !frame.minimized;
  const interactionStore = useStore(osWindowInteractionLogic, { visible: visibleNow });
  const interaction = useSelector(interactionStore, snapshot => snapshot.context);

  useEffect(() => {
    interactionStore.trigger.visibilityObserved({ visible: visibleNow });
  }, [interactionStore, visibleNow]);

  useEffect(() => () => interactionStore.trigger.disposed(), [interactionStore]);

  useEffect(() => {
    if (!gestureActive) return;
    return bindLayoutGestureCancellation(window, (reason, point) => {
      windowManagerStore.trigger.gestureCancelled({
        reason,
        ...(point ? { point } : {}),
      });
    });
  }, [gestureActive]);

  const candidates = (): OccupiedSnapCandidate[] => {
    const state = manager.getState();
    return Object.values(state.windows).flatMap(window =>
      memberIds.has(window.id) ||
      window.desktopId !== frame.desktopId ||
      window.minimized ||
      window.nodeId === null
        ? []
        : [
            {
              windowId: window.id,
              nodeId: window.nodeId,
              rect: window.rect,
              z: window.layer,
              parentAxis: window.parentAxis,
            },
          ]
    );
  };

  const targetAt = (
    point: { x: number; y: number },
    capturedWorkArea: OsRect,
    swapModifierActive: boolean
  ) => {
    const config = snapTargetConfigFromConfig(manager.getState().windowManagerConfig);
    if (config === null) return null;
    return resolveSnapTarget({
      point,
      workArea: capturedWorkArea,
      candidates: candidates(),
      currentTarget: windowManagerStoreGesture() ?? undefined,
      config,
      swapModifierActive,
    });
  };

  const swapModifierHeld = (event: OsDragEvent): boolean => {
    const modifier = manager.getState().windowManagerConfig?.swapModifier;
    return modifier !== undefined && modifier !== "none" && modifierActive(event, modifier);
  };

  const dragPreview = useAnimationFrameLatest<PendingDragPreview>(pending => {
    const snapshot = windowManagerStore.getSnapshot().context;
    const gesture = snapshot.gesture;
    const currentWorkArea = snapshot.workArea?.rect;
    if (
      gesture?.status !== "active" ||
      gesture.source.windowId !== activeId ||
      currentWorkArea === undefined
    ) {
      return;
    }
    windowManagerStore.trigger.gesturePreviewed({
      point: pending.point,
      preview:
        snapshot.deckDropTarget !== null
          ? null
          : targetAt(pending.point, gesture.workArea, pending.swapModifierActive),
      currentWorkArea,
    });
  });

  const snapBackToAuthoritative = () => {
    const authoritative = manager.getState().windows[activeId];
    if (authoritative) {
      interactionStore.trigger.authoritativeRectApplied({ rect: authoritative.rect });
    }
  };

  const scheduleAuthoritativeRollback = (gestureAttempt: number) => {
    interactionStore.trigger.rollbackRequested({ attempt: gestureAttempt });
  };

  useLayoutEffect(() => {
    const gestureAttempt = interaction.rollbackAttempt;
    if (gestureAttempt === null || interaction.gestureAttempt !== gestureAttempt) return;
    interactionStore.trigger.rollbackApplied({ attempt: gestureAttempt });
    const authoritative = manager.getState().windows[activeId];
    if (!authoritative) return;
    interactionStore.trigger.authoritativeRectApplied({ rect: authoritative.rect });
  }, [
    activeId,
    interaction.gestureAttempt,
    interaction.rollbackAttempt,
    interactionStore,
    manager,
  ]);

  // Holds the released rect while the commit is in flight; the settled
  // authoritative rect replaces it, and rollbacks clear it explicitly.
  const trackGestureOutcome = (
    rect: OsRect,
    outcome: WindowManagerCommandOutcome,
    gestureAttempt: number
  ) => {
    if (!outcome.accepted) {
      scheduleAuthoritativeRollback(gestureAttempt);
      return;
    }
    interactionStore.trigger.rectHeld({ attempt: gestureAttempt, rect });
    void outcome.completion.then(
      applied => {
        interactionStore.trigger.gestureCompletionSettled({ attempt: gestureAttempt });
        if (!applied) scheduleAuthoritativeRollback(gestureAttempt);
      },
      error => {
        interactionStore.trigger.gestureCompletionSettled({ attempt: gestureAttempt });
        scheduleAuthoritativeRollback(gestureAttempt);
        console.error("Window gesture command failed", error);
      }
    );
  };

  // Escape cancelled the gesture: snap back to the source rect instead of tracking the pointer.
  const snapBackCancelled = (): boolean => {
    const gesture = windowManagerStore.getSnapshot().context.gesture;
    if (gesture?.status !== "cancelled" || gesture.source.windowId !== activeId) return false;
    snapBackToAuthoritative();
    return true;
  };

  const settleUncommittedDrag = () => {
    const gestureAttempt = interactionStore.getSnapshot().context.gestureAttempt;
    windowManagerStore.trigger.gestureCleared();
    scheduleAuthoritativeRollback(gestureAttempt);
  };

  const settleCancelledDrag = (): boolean => {
    const gesture = windowManagerStore.getSnapshot().context.gesture;
    if (gesture?.status !== "cancelled" || gesture.source.windowId !== activeId) return false;
    settleUncommittedDrag();
    return true;
  };

  const activate = (target: EventTarget | null) => {
    const state = manager.getState();
    const win = state.windows[activeId];
    if (!win || (focused && !win.minimized)) return;
    const element = target instanceof Element ? target : null;
    void coordinator.userFocus(activeId, { viaLink: Boolean(element?.closest("a[href]")) });
  };

  const handleDragStart: RndDragCallback = (event, _data) => {
    const point = layerPoint(event);
    const state = manager.getState();
    const config = state.windowManagerConfig;
    const win = state.windows[activeId];
    if (!point || !workArea || !win || !state.snapshot || config === null) return;
    interactionStore.trigger.gestureAttemptStarted();
    const moveGroup =
      movesAsUnit ||
      config.dragAwayPolicy === "group" ||
      modifierActive(event, config.groupMoveModifier);
    windowManagerStore.trigger.gestureBegan({
      pointerId: 0,
      point,
      workArea: workArea.rect,
      layoutRevision: state.snapshot.revision,
      source: {
        windowId: activeId,
        nodeId: win.nodeId,
        groupId: win.groupId,
        moveMode: moveGroup ? "group" : "window",
      },
    });
  };

  const handleDrag: RndDragCallback = (event, _data) => {
    if (snapBackCancelled()) {
      dragPreview.cancel();
      return;
    }
    const point = layerPoint(event);
    const gesture = windowManagerStore.getSnapshot().context.gesture;
    const currentWorkArea = windowManagerStore.getSnapshot().context.workArea?.rect;
    if (!point || gesture?.status !== "active" || !currentWorkArea) return;
    // Pointer streams can outpace paint by orders of magnitude. Keep only the
    // latest point for the next frame so snap projection and shell subscribers
    // never do duplicate work that cannot be displayed.
    dragPreview.schedule({
      value: {
        point,
        swapModifierActive: swapModifierHeld(event),
      },
    });
  };

  const handleDragStop: RndDragCallback = (event, data) => {
    dragPreview.cancel();
    if (settleCancelledDrag()) return;
    const point = layerPoint(event) ?? { x: data.x, y: data.y };
    const state = manager.getState();
    const currentGesture = windowManagerStore.getSnapshot().context.gesture;
    const currentWorkArea = windowManagerStore.getSnapshot().context.workArea?.rect;
    const win = state.windows[activeId];
    if (!win || !state.snapshot || currentGesture?.status !== "active" || !currentWorkArea) {
      settleUncommittedDrag();
      return;
    }
    // A deck advertising itself as the drop target wins over snap targets:
    // the release commits ONE `window.stack.group{insert_index}` folding every
    // member of the dragged frame in order (US-001.EC-1, US-014).
    const dropTarget = windowManagerStore.getSnapshot().context.deckDropTarget;
    if (dropTarget !== null) {
      const gestureAttempt = interactionStore.getSnapshot().context.gestureAttempt;
      windowManagerStore.trigger.gestureCleared();
      const outcome = manager.groupWindows(
        dropTarget.targetWindowId,
        frame.members,
        dropTarget.insertIndex
      );
      trackGestureOutcome({ ...win.rect, x: data.x, y: data.y }, outcome, gestureAttempt);
      return;
    }
    const target = targetAt(point, currentGesture.workArea, swapModifierHeld(event));
    const decision = finishWindowManagerGesture(windowManagerStore, {
      finalPoint: point,
      finalTarget: target,
      currentRevision: state.snapshot.revision,
      currentWorkArea,
      clientValid: state.client !== null,
    });
    if (decision?.kind === "commit") {
      const gestureAttempt = interactionStore.getSnapshot().context.gestureAttempt;
      const outcome = manager.applySnapTarget(
        activeId,
        decision.command.target,
        decision.command.source.moveMode === "group"
      );
      trackGestureOutcome(decision.command.target.rect, outcome, gestureAttempt);
      return;
    }
    if (decision?.kind === "cancel" && decision.reason !== "no-target") {
      settleCancelledDrag();
      return;
    }
    if (decision?.reason === "no-target") {
      const gestureAttempt = interactionStore.getSnapshot().context.gestureAttempt;
      const dropped = clampFloatingRect({
        proposedRect: { ...win.rect, x: data.x, y: data.y },
        workArea: currentWorkArea,
        pointer: point,
        grabOffset: {
          x: Math.max(0, point.x - data.x),
          y: Math.max(0, point.y - data.y),
        },
      });
      const outcome = manager.commitFloatingRect(activeId, dropped, undefined, movesAsUnit);
      trackGestureOutcome(dropped, outcome, gestureAttempt);
    }
    windowManagerStore.trigger.gestureCleared();
  };

  const resize = useOsWindowResize(frame, {
    manager,
    activeId,
    movesAsUnit,
    nextGestureAttempt: () => {
      interactionStore.trigger.gestureAttemptStarted();
      return interactionStore.getSnapshot().context.gestureAttempt;
    },
    resizeMax: interaction.resizeMax,
    setResizeMax: maximum => interactionStore.trigger.resizeMaximumChanged({ maximum }),
    trackGestureOutcome,
    scheduleAuthoritativeRollback,
  });

  return {
    focused,
    keepMounted: interaction.hasBeenVisible || visibleNow,
    rect: interaction.pendingRect ?? frame.rect,
    registerRnd: instance => {
      interactionStore.trigger.rndRegistered({ instance });
    },
    resizeMax: resize.resizeMax,
    handleTrafficLight: action => {
      if (action === "close") {
        if (frame.members.length > 1) void manager.closeWindowScoped(activeId, "group");
        else void coordinator.userClose(activeId);
      } else if (action === "minimize") void coordinator.userMinimize(activeId);
      else void coordinator.userZoom(activeId);
    },
    handlePointerEnter: () => {
      const state = manager.getState();
      if (state.windowManagerConfig?.focusFollowsPointer) {
        void coordinator.userFocus(activeId);
      }
    },
    handlePointerDownCapture: event => {
      if (manager.getState().windowManagerConfig?.focusPolicy === "click_directional") {
        activate(event.target);
      }
    },
    handleFocusCapture: event => activate(event.target),
    handleDragStart,
    handleDrag,
    handleDragStop,
    handleResizeStart: resize.handleResizeStart,
    handleResizeStop: resize.handleResizeStop,
  };
}

function windowManagerStoreGesture() {
  const session = windowManagerStore.getSnapshot().context.gesture;
  return session?.status === "active" ? session.preview : null;
}
