import { createStoreLogic } from "@xstate/store";

const DRAG_START_SLOP = 4;

interface PointerPoint {
  clientX: number;
  clientY: number;
}

export interface OsDeckTabDrag {
  windowId: string;
  mode: "reorder" | "out";
  /** Insertion slot (0..members.length) while reordering; null once outside. */
  insertIndex: number | null;
}

export interface OsWindowDeckDragRuntime {
  commit: (windowId: string, point: PointerPoint) => void;
  project: (windowId: string, point: PointerPoint) => OsDeckTabDrag;
}

interface PressedPointer {
  start: PointerPoint;
  windowId: string;
}

interface DeckDragIdleContext {
  dragged: boolean;
  frameScheduled: false;
  generation: number;
  pendingPoint: null;
  phase: "idle";
  press: null;
  preview: null;
}

interface DeckDragPressedContext {
  dragged: false;
  frameScheduled: boolean;
  generation: number;
  pendingPoint: PointerPoint | null;
  phase: "pressed";
  press: PressedPointer;
  preview: null;
}

interface DeckDragActiveContext {
  dragged: true;
  frameScheduled: boolean;
  generation: number;
  pendingPoint: PointerPoint | null;
  phase: "dragging";
  press: PressedPointer;
  preview: OsDeckTabDrag;
}

type OsWindowDeckDragContext = DeckDragIdleContext | DeckDragPressedContext | DeckDragActiveContext;

type OsWindowDeckDragEvents = {
  animationFrameElapsed: { generation: number };
  disposed: Record<never, never>;
  pointerCancelled: { generation: number };
  pointerPressed: {
    point: PointerPoint;
    runtime: OsWindowDeckDragRuntime;
    windowId: string;
  };
  pointerReleased: { generation: number; point: PointerPoint };
  pointerSampled: { generation: number; point: PointerPoint };
  previewProjected: {
    generation: number;
    preview: OsDeckTabDrag;
  };
};

interface DeckDragTrigger {
  animationFrameElapsed: (event: OsWindowDeckDragEvents["animationFrameElapsed"]) => void;
  pointerCancelled: (event: OsWindowDeckDragEvents["pointerCancelled"]) => void;
  pointerReleased: (event: OsWindowDeckDragEvents["pointerReleased"]) => void;
  pointerSampled: (event: OsWindowDeckDragEvents["pointerSampled"]) => void;
  previewProjected: (event: OsWindowDeckDragEvents["previewProjected"]) => void;
}

interface DeckDragHandles {
  animationFrameId: number | null;
  runtime: OsWindowDeckDragRuntime;
  unbind: () => void;
}

const handlesByTrigger = new WeakMap<object, DeckDragHandles>();

function movedBeyondSlop(start: PointerPoint, point: PointerPoint): boolean {
  return (
    Math.abs(point.clientX - start.clientX) >= DRAG_START_SLOP ||
    Math.abs(point.clientY - start.clientY) >= DRAG_START_SLOP
  );
}

function idleContext(generation: number, dragged: boolean): DeckDragIdleContext {
  return {
    dragged,
    frameScheduled: false,
    generation,
    pendingPoint: null,
    phase: "idle",
    press: null,
    preview: null,
  };
}

function cleanupHandles(handles: DeckDragHandles): void {
  if (handles.animationFrameId !== null) cancelAnimationFrame(handles.animationFrameId);
  handles.animationFrameId = null;
  handles.unbind();
}

function releaseHandles(trigger: object): DeckDragHandles | null {
  const handles = handlesByTrigger.get(trigger);
  if (!handles) return null;
  cleanupHandles(handles);
  handlesByTrigger.delete(trigger);
  return handles;
}

function bindPointerLifecycle(trigger: DeckDragTrigger, generation: number): () => void {
  const onPointerMove = (event: PointerEvent) =>
    trigger.pointerSampled({
      generation,
      point: { clientX: event.clientX, clientY: event.clientY },
    });
  const onPointerUp = (event: PointerEvent) =>
    trigger.pointerReleased({
      generation,
      point: { clientX: event.clientX, clientY: event.clientY },
    });
  const onPointerCancel = () => trigger.pointerCancelled({ generation });
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Escape") trigger.pointerCancelled({ generation });
  };
  window.addEventListener("pointermove", onPointerMove);
  window.addEventListener("pointerup", onPointerUp);
  window.addEventListener("pointercancel", onPointerCancel);
  window.addEventListener("keydown", onKeyDown);
  return () => {
    window.removeEventListener("pointermove", onPointerMove);
    window.removeEventListener("pointerup", onPointerUp);
    window.removeEventListener("pointercancel", onPointerCancel);
    window.removeEventListener("keydown", onKeyDown);
  };
}

export const osWindowDeckDragLogic = createStoreLogic<
  OsWindowDeckDragContext,
  OsWindowDeckDragEvents
>({
  context: idleContext(0, false),
  on: {
    pointerPressed: (context, event, enqueue) => {
      const generation = context.generation + 1;
      enqueue.effect(({ trigger }) => {
        releaseHandles(trigger);
        handlesByTrigger.set(trigger, {
          animationFrameId: null,
          runtime: event.runtime,
          unbind: bindPointerLifecycle(trigger, generation),
        });
      });
      return {
        dragged: false,
        frameScheduled: false,
        generation,
        pendingPoint: null,
        phase: "pressed",
        press: { start: event.point, windowId: event.windowId },
        preview: null,
      };
    },
    pointerSampled: (context, event, enqueue) => {
      if (context.phase === "idle" || event.generation !== context.generation) return;
      if (!context.frameScheduled) {
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (!handles) return;
          handles.animationFrameId = requestAnimationFrame(() =>
            trigger.animationFrameElapsed({ generation: context.generation })
          );
        });
      }
      return { ...context, frameScheduled: true, pendingPoint: event.point };
    },
    animationFrameElapsed: (context, event, enqueue) => {
      if (
        context.phase === "idle" ||
        event.generation !== context.generation ||
        context.pendingPoint === null
      ) {
        return;
      }
      const point = context.pendingPoint;
      enqueue.effect(({ trigger }) => {
        const handles = handlesByTrigger.get(trigger);
        if (!handles) return;
        handles.animationFrameId = null;
        if (!movedBeyondSlop(context.press.start, point)) return;
        try {
          trigger.previewProjected({
            generation: context.generation,
            preview: handles.runtime.project(context.press.windowId, point),
          });
        } catch (error) {
          console.error("Failed to project a window-deck drag", error);
          trigger.pointerCancelled({ generation: context.generation });
        }
      });
      return { ...context, frameScheduled: false, pendingPoint: null };
    },
    previewProjected: (context, event) => {
      if (context.phase === "idle" || event.generation !== context.generation) return;
      return {
        ...context,
        dragged: true,
        phase: "dragging",
        preview: event.preview,
      };
    },
    pointerReleased: (context, event, enqueue) => {
      if (context.phase === "idle" || event.generation !== context.generation) return;
      const dragged =
        context.phase === "dragging" || movedBeyondSlop(context.press.start, event.point);
      enqueue.effect(({ trigger }) => {
        const handles = releaseHandles(trigger);
        if (!handles) return;
        if (!dragged) return;
        try {
          handles.runtime.commit(context.press.windowId, event.point);
        } catch (error) {
          console.error("Failed to commit a window-deck drag", error);
        }
      });
      return idleContext(context.generation, dragged);
    },
    pointerCancelled: (context, event, enqueue) => {
      if (context.phase === "idle" || event.generation !== context.generation) return;
      const dragged =
        context.phase === "dragging" ||
        (context.pendingPoint !== null &&
          movedBeyondSlop(context.press.start, context.pendingPoint));
      enqueue.effect(({ trigger }) => {
        releaseHandles(trigger);
      });
      return idleContext(context.generation, dragged);
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        releaseHandles(trigger);
      });
      return idleContext(context.generation + 1, false);
    },
  },
});
