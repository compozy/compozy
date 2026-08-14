import { createStoreLogic } from "@xstate/store";

const CLOSE_DELAY_MS = 120;

interface WorktreeHoverSubmenuContext {
  generation: number;
  phase: "idle" | "closing" | "focus-pending";
}

type WorktreeHoverSubmenuEvents = {
  interactionEntered: { open: () => void };
  closeScheduled: { close: () => void };
  closeElapsed: { close: () => void; generation: number };
  keyboardOpenRequested: { open: () => void };
  panelFocused: {};
  keyboardClosed: { close: () => void; returnFocus: () => void };
  disposed: {};
};

const closeTimers = new WeakMap<object, number>();

function clearCloseTimer(trigger: object) {
  const timer = closeTimers.get(trigger);
  if (timer === undefined) return;
  window.clearTimeout(timer);
  closeTimers.delete(trigger);
}

export const worktreeHoverSubmenuLogic = createStoreLogic<
  WorktreeHoverSubmenuContext,
  WorktreeHoverSubmenuEvents
>({
  context: { generation: 0, phase: "idle" },
  on: {
    interactionEntered: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        clearCloseTimer(trigger);
        event.open();
      });
      return {
        ...context,
        phase: context.phase === "focus-pending" ? "focus-pending" : "idle",
      };
    },
    closeScheduled: (context, event, enqueue) => {
      const generation = context.generation + 1;
      enqueue.effect(({ trigger }) => {
        clearCloseTimer(trigger);
        const timer = window.setTimeout(() => {
          closeTimers.delete(trigger);
          trigger.closeElapsed({ close: event.close, generation });
        }, CLOSE_DELAY_MS);
        closeTimers.set(trigger, timer);
      });
      return { generation, phase: "closing" };
    },
    closeElapsed: (context, event, enqueue) => {
      if (context.phase !== "closing" || event.generation !== context.generation) return;
      enqueue.effect(event.close);
      return { ...context, phase: "idle" };
    },
    keyboardOpenRequested: (_context, event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        clearCloseTimer(trigger);
        event.open();
      });
      return { generation: _context.generation + 1, phase: "focus-pending" };
    },
    panelFocused: context => ({ ...context, phase: "idle" }),
    keyboardClosed: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        clearCloseTimer(trigger);
        event.close();
        event.returnFocus();
      });
      return { generation: context.generation + 1, phase: "idle" };
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => clearCloseTimer(trigger));
      return { generation: context.generation + 1, phase: "idle" };
    },
  },
});
