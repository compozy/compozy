import { createStoreLogic } from "@xstate/store";

interface SeamDragState<TSeam> {
  coordinate: number;
  pendingDeltaPx: number;
  phase: "idle" | "dragging";
  pointerId: number | null;
  seam: TSeam | null;
}

type SeamDragEvents<TSeam> = {
  disposed: { endPreview: () => void };
  dragCancelled: { endPreview: () => void };
  dragEnded: { pointerId: number };
  dragStarted: { coordinate: number; pointerId: number; seam: TSeam };
  pointerMoved: {
    coordinate: number;
    pointerId: number;
    preview: (seam: TSeam, deltaPx: number) => void;
  };
  previewElapsed: Record<never, never>;
};

interface SeamDragHandles {
  frame: number | null;
  preview: ((seam: unknown, deltaPx: number) => void) | null;
}

const handlesByTrigger = new WeakMap<object, SeamDragHandles>();

function handlesFor(trigger: object): SeamDragHandles {
  const existing = handlesByTrigger.get(trigger);
  if (existing) return existing;
  const created = { frame: null, preview: null };
  handlesByTrigger.set(trigger, created);
  return created;
}

function cancelFrame(handles: SeamDragHandles): void {
  if (handles.frame !== null) cancelAnimationFrame(handles.frame);
  handles.frame = null;
  handles.preview = null;
}

function invoke(operation: () => void, label: string): void {
  try {
    operation();
  } catch (error) {
    console.error(label, error);
  }
}

export function createSeamDragLogic<TSeam>() {
  return createStoreLogic<SeamDragState<TSeam>, SeamDragEvents<TSeam>>({
    context: {
      coordinate: 0,
      pendingDeltaPx: 0,
      phase: "idle",
      pointerId: null,
      seam: null,
    },
    on: {
      dragStarted: (_context, event) => ({
        coordinate: event.coordinate,
        pendingDeltaPx: 0,
        phase: "dragging",
        pointerId: event.pointerId,
        seam: event.seam,
      }),
      pointerMoved: (context, event, enqueue) => {
        if (context.phase !== "dragging" || context.pointerId !== event.pointerId) return;
        const pendingDeltaPx = event.coordinate - context.coordinate;
        enqueue.effect(({ trigger }) => {
          const handles = handlesFor(trigger);
          handles.preview = event.preview as (seam: unknown, deltaPx: number) => void;
          if (handles.frame !== null) return;
          handles.frame = requestAnimationFrame(() => {
            handles.frame = null;
            trigger.previewElapsed();
          });
        });
        return { ...context, pendingDeltaPx };
      },
      previewElapsed: (context, _event, enqueue) => {
        if (context.phase !== "dragging" || context.seam === null) return;
        const seam = context.seam;
        enqueue.effect(({ trigger }) => {
          const preview = handlesFor(trigger).preview;
          if (preview) {
            invoke(
              () => preview(seam, context.pendingDeltaPx),
              "Failed to render a window seam preview"
            );
          }
        });
      },
      dragEnded: (context, event, enqueue) => {
        if (context.phase !== "dragging" || context.pointerId !== event.pointerId) return;
        enqueue.effect(({ trigger }) => cancelFrame(handlesFor(trigger)));
        return {
          coordinate: 0,
          pendingDeltaPx: 0,
          phase: "idle",
          pointerId: null,
          seam: null,
        };
      },
      dragCancelled: (context, event, enqueue) => {
        if (context.phase !== "dragging") return;
        enqueue.effect(({ trigger }) => {
          cancelFrame(handlesFor(trigger));
          invoke(event.endPreview, "Failed to end a cancelled window seam preview");
        });
        return { ...context, phase: "idle", pointerId: null, seam: null };
      },
      disposed: (context, event, enqueue) => {
        enqueue.effect(({ trigger }) => {
          const handles = handlesByTrigger.get(trigger);
          if (handles) cancelFrame(handles);
          handlesByTrigger.delete(trigger);
          if (context.phase === "dragging") {
            invoke(event.endPreview, "Failed to end a disposed window seam preview");
          }
        });
        return { ...context, phase: "idle", pointerId: null, seam: null };
      },
    },
  });
}
