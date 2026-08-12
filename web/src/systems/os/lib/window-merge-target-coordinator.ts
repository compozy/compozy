import type { OsWindowFrameModel } from "./group-projection";
import type { OsDesktopRuntimeStore, OsRect, WindowManagerController } from "./os-types";
import { autoScrollDeckAtEdges, insertSlotAt, pointInsideDeck } from "./window-deck-geometry";
import { windowManagerStore } from "../stores/window-manager-store";

const HEAD_HIT_SLOP_TOP = 4;
const HEAD_HIT_SLOP_BOTTOM = 8;

interface MergeTargetRegistrationBase {
  frameId: string;
  getFrame: () => OsWindowFrameModel;
}

interface HeadMergeTargetRegistration extends MergeTargetRegistrationBase {
  kind: "head";
  getChrome: () => HTMLElement | null;
}

interface DeckMergeTargetRegistration extends MergeTargetRegistrationBase {
  kind: "deck";
  getTabElements: () => ReadonlyMap<string, HTMLElement>;
  getTabs: () => HTMLElement | null;
}

type MergeTargetRegistration = HeadMergeTargetRegistration | DeckMergeTargetRegistration;

interface MergeTargetCoordinator {
  frameID: number | null;
  pending: { clientX: number; clientY: number } | null;
  pointerListening: boolean;
  registrations: Map<string, MergeTargetRegistration>;
  onPointerMove: (event: PointerEvent) => void;
  unsubscribeStore: () => void;
}

const coordinators = new WeakMap<WindowManagerController, MergeTargetCoordinator>();

function pointInRect(point: { x: number; y: number }, rect: OsRect): boolean {
  return (
    point.x >= rect.x &&
    point.x <= rect.x + rect.w &&
    point.y >= rect.y &&
    point.y <= rect.y + rect.h
  );
}

function highestFloatingLayerAtPoint(
  state: OsDesktopRuntimeStore,
  draggedWindowId: string,
  layerPoint: { x: number; y: number }
): ReadonlyMap<string, number> {
  const layers = new Map<string, number>();
  for (const [desktopId, frames] of Object.entries(state.frames)) {
    for (const frame of frames) {
      if (
        frame.kind !== "floating" ||
        frame.minimized ||
        frame.members.some(member => member === draggedWindowId) ||
        !pointInRect(layerPoint, frame.rect)
      ) {
        continue;
      }
      const current = layers.get(desktopId);
      if (current === undefined || frame.layer > current) layers.set(desktopId, frame.layer);
    }
  }
  return layers;
}

function pointCanReachFrameHead(point: { x: number; y: number }, rect: OsRect): boolean {
  return (
    point.x >= rect.x &&
    point.x <= rect.x + rect.w &&
    point.y >= rect.y - HEAD_HIT_SLOP_TOP &&
    point.y <= rect.y + rect.h
  );
}

function resolveMergeTarget(
  manager: WindowManagerController,
  registrations: Iterable<MergeTargetRegistration>,
  point: { clientX: number; clientY: number }
) {
  const storeContext = windowManagerStore.getSnapshot().context;
  const gesture = storeContext.gesture;
  if (gesture?.status !== "active") return null;

  const state = manager.getState();
  const origin = storeContext.workArea?.origin ?? { x: 0, y: 0 };
  const layerPoint = { x: point.clientX - origin.x, y: point.clientY - origin.y };
  const candidates: Array<{
    registration: MergeTargetRegistration;
    frame: OsWindowFrameModel;
  }> = [];
  for (const registration of registrations) {
    const frame = registration.getFrame();
    if (!frame.members.some(member => member === gesture.source.windowId)) {
      candidates.push({ registration, frame });
    }
  }
  candidates.sort((left, right) => right.frame.layer - left.frame.layer);
  const highestLayers = highestFloatingLayerAtPoint(state, gesture.source.windowId, layerPoint);

  for (const { registration, frame } of candidates) {
    if (!pointCanReachFrameHead(layerPoint, frame.rect)) continue;
    const covered = (highestLayers.get(frame.desktopId) ?? frame.layer) > frame.layer;
    if (covered) continue;
    if (registration.kind === "deck") {
      const tabs = registration.getTabs();
      const bounds = tabs?.getBoundingClientRect();
      if (
        tabs === null ||
        bounds === undefined ||
        !pointInsideDeck(
          tabs,
          point,
          {
            top: HEAD_HIT_SLOP_TOP,
            bottom: HEAD_HIT_SLOP_BOTTOM,
          },
          bounds
        )
      ) {
        continue;
      }
      autoScrollDeckAtEdges(tabs, point.clientX, bounds);
      return {
        frameId: frame.id,
        targetWindowId: frame.activeWindowId,
        insertIndex: insertSlotAt(frame.members, registration.getTabElements(), point.clientX),
      };
    }
    const head = registration.getChrome()?.querySelector('[data-slot="os-window-head"]');
    if (!(head instanceof HTMLElement)) continue;
    const box = head.getBoundingClientRect();
    const inside =
      point.clientX >= box.left &&
      point.clientX <= box.right &&
      point.clientY >= box.top - HEAD_HIT_SLOP_TOP &&
      point.clientY <= box.bottom + HEAD_HIT_SLOP_BOTTOM;
    if (!inside) continue;
    return {
      frameId: frame.id,
      targetWindowId: frame.activeWindowId,
      insertIndex: frame.members.length,
    };
  }
  return null;
}

function publishMergeTarget(manager: WindowManagerController, coordinator: MergeTargetCoordinator) {
  const point = coordinator.pending;
  coordinator.pending = null;
  if (point === null) return;
  const target = resolveMergeTarget(manager, coordinator.registrations.values(), point);
  const current = windowManagerStore.getSnapshot().context.deckDropTarget;
  if (target === null) {
    if (current !== null && coordinator.registrations.has(current.frameId)) {
      windowManagerStore.trigger.deckDropCleared({ frameId: current.frameId });
    }
    return;
  }
  if (
    current?.frameId === target.frameId &&
    current.targetWindowId === target.targetWindowId &&
    current.insertIndex === target.insertIndex
  ) {
    return;
  }
  windowManagerStore.trigger.deckDropTargeted({ target });
}

function setPointerListening(coordinator: MergeTargetCoordinator, listening: boolean): void {
  if (coordinator.pointerListening === listening) return;
  coordinator.pointerListening = listening;
  if (listening) {
    window.addEventListener("pointermove", coordinator.onPointerMove);
    return;
  }
  window.removeEventListener("pointermove", coordinator.onPointerMove);
  coordinator.pending = null;
  if (coordinator.frameID !== null) cancelAnimationFrame(coordinator.frameID);
  coordinator.frameID = null;
}

function createCoordinator(manager: WindowManagerController): MergeTargetCoordinator {
  const coordinator: MergeTargetCoordinator = {
    frameID: null,
    pending: null,
    pointerListening: false,
    registrations: new Map(),
    onPointerMove: event => {
      coordinator.pending = { clientX: event.clientX, clientY: event.clientY };
      if (coordinator.frameID !== null) return;
      coordinator.frameID = requestAnimationFrame(() => {
        coordinator.frameID = null;
        publishMergeTarget(manager, coordinator);
      });
    },
    unsubscribeStore: () => {},
  };
  const subscription = windowManagerStore.subscribe(snapshot => {
    setPointerListening(coordinator, snapshot.context.gesture?.status === "active");
  });
  coordinator.unsubscribeStore = () => subscription.unsubscribe();
  setPointerListening(
    coordinator,
    windowManagerStore.getSnapshot().context.gesture?.status === "active"
  );
  return coordinator;
}

/** Registers one solo frame with the gesture-wide merge hit-test coordinator. */
export function registerWindowMergeTarget(
  manager: WindowManagerController,
  registration: Omit<HeadMergeTargetRegistration, "kind">
): () => void {
  return registerMergeTarget(manager, { ...registration, kind: "head" });
}

/** Registers one tab deck with the shared, frame-rate-bounded merge hit tester. */
export function registerWindowDeckMergeTarget(
  manager: WindowManagerController,
  registration: Omit<DeckMergeTargetRegistration, "kind">
): () => void {
  return registerMergeTarget(manager, { ...registration, kind: "deck" });
}

function registerMergeTarget(
  manager: WindowManagerController,
  registration: MergeTargetRegistration
): () => void {
  let coordinator = coordinators.get(manager);
  if (coordinator === undefined) {
    coordinator = createCoordinator(manager);
    coordinators.set(manager, coordinator);
  }
  coordinator.registrations.set(registration.frameId, registration);

  return () => {
    const activeCoordinator = coordinators.get(manager);
    if (activeCoordinator === undefined) return;
    activeCoordinator.registrations.delete(registration.frameId);
    windowManagerStore.trigger.deckDropCleared({ frameId: registration.frameId });
    if (activeCoordinator.registrations.size > 0) return;
    setPointerListening(activeCoordinator, false);
    activeCoordinator.unsubscribeStore();
    coordinators.delete(manager);
  };
}
