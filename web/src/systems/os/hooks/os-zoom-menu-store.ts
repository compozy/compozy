import { createStoreLogic } from "@xstate/store";

/** Hover intent before the zoom menu opens (macOS green-button posture). */
export const OS_ZOOM_MENU_OPEN_DELAY_MS = 250;
/** Grace period crossing from the button into the menu before it closes. */
export const OS_ZOOM_MENU_CLOSE_GRACE_MS = 300;

type OsZoomMenuPhase = "closed" | "opening" | "open" | "closing";

interface OsZoomMenuState {
  generation: number;
  phase: OsZoomMenuPhase;
}

type OsZoomMenuEvents = {
  closeGraceElapsed: { generation: number };
  contentEntered: Record<never, never>;
  dismissed: Record<never, never>;
  disposed: Record<never, never>;
  hoverEntered: Record<never, never>;
  hoverLeft: Record<never, never>;
  openChanged: { open: boolean };
  openDelayElapsed: { generation: number };
};

interface ZoomMenuTimers {
  close: ReturnType<typeof setTimeout> | null;
  open: ReturnType<typeof setTimeout> | null;
}

interface ZoomMenuTrigger {
  closeGraceElapsed: (event: OsZoomMenuEvents["closeGraceElapsed"]) => void;
  openDelayElapsed: (event: OsZoomMenuEvents["openDelayElapsed"]) => void;
}

const timersByTrigger = new WeakMap<object, ZoomMenuTimers>();

function timersFor(trigger: object): ZoomMenuTimers {
  const existing = timersByTrigger.get(trigger);
  if (existing) return existing;
  const created = { close: null, open: null };
  timersByTrigger.set(trigger, created);
  return created;
}

function clearTimer(timer: ReturnType<typeof setTimeout> | null): void {
  if (timer !== null) clearTimeout(timer);
}

function clearTimers(trigger: object): void {
  const timers = timersByTrigger.get(trigger);
  if (!timers) return;
  clearTimer(timers.open);
  clearTimer(timers.close);
  timers.open = null;
  timers.close = null;
}

function scheduleOpen(trigger: ZoomMenuTrigger, generation: number): void {
  const timers = timersFor(trigger);
  clearTimer(timers.open);
  timers.open = setTimeout(
    () => trigger.openDelayElapsed({ generation }),
    OS_ZOOM_MENU_OPEN_DELAY_MS
  );
}

function scheduleClose(trigger: ZoomMenuTrigger, generation: number): void {
  const timers = timersFor(trigger);
  clearTimer(timers.close);
  timers.close = setTimeout(
    () => trigger.closeGraceElapsed({ generation }),
    OS_ZOOM_MENU_CLOSE_GRACE_MS
  );
}

export const osZoomMenuLogic = createStoreLogic<OsZoomMenuState, OsZoomMenuEvents>({
  context: { generation: 0, phase: "closed" },
  on: {
    hoverEntered: (context, _event, enqueue) => {
      if (context.phase === "open" || context.phase === "opening") return;
      const generation = context.generation + 1;
      enqueue.effect(({ trigger }) => {
        clearTimers(trigger);
        if (context.phase === "closed") scheduleOpen(trigger, generation);
      });
      return {
        generation,
        phase: context.phase === "closing" ? "open" : "opening",
      };
    },
    hoverLeft: (context, _event, enqueue) => {
      if (context.phase === "closed" || context.phase === "closing") return;
      const generation = context.generation + 1;
      enqueue.effect(({ trigger }) => {
        clearTimers(trigger);
        if (context.phase === "open") scheduleClose(trigger, generation);
      });
      return {
        generation,
        phase: context.phase === "opening" ? "closed" : "closing",
      };
    },
    contentEntered: (context, _event, enqueue) => {
      if (context.phase !== "closing") return;
      enqueue.effect(({ trigger }) => clearTimers(trigger));
      return { generation: context.generation + 1, phase: "open" };
    },
    openDelayElapsed: (context, event) =>
      context.phase === "opening" && event.generation === context.generation
        ? { ...context, phase: "open" }
        : undefined,
    closeGraceElapsed: (context, event) =>
      context.phase === "closing" && event.generation === context.generation
        ? { ...context, phase: "closed" }
        : undefined,
    openChanged: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => clearTimers(trigger));
      return {
        generation: context.generation + 1,
        phase: event.open ? "open" : "closed",
      };
    },
    dismissed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => clearTimers(trigger));
      return { generation: context.generation + 1, phase: "closed" };
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        clearTimers(trigger);
        timersByTrigger.delete(trigger);
      });
      return { generation: context.generation + 1, phase: "closed" };
    },
  },
});
