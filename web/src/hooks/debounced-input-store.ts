import { createStoreLogic } from "@xstate/store";

interface DebouncedInputState {
  committedValue: string;
  draftValue: string;
  externalValue: string;
  generation: number;
  phase: "idle" | "waiting";
}

type DebouncedInputEvents = {
  cleared: { commit: (value: string) => void };
  commitElapsed: { generation: number };
  disposed: Record<never, never>;
  externalValueObserved: { value: string };
  reset: { value: string };
  valueChanged: {
    commit: (value: string) => void;
    delayMs: number;
    value: string;
  };
};

interface DebouncedInputHandles {
  commit: ((value: string) => void) | null;
  timer: ReturnType<typeof setTimeout> | null;
}

const handlesByTrigger = new WeakMap<object, DebouncedInputHandles>();

function handlesFor(trigger: object): DebouncedInputHandles {
  const existing = handlesByTrigger.get(trigger);
  if (existing) return existing;
  const created = { commit: null, timer: null };
  handlesByTrigger.set(trigger, created);
  return created;
}

function clearPending(handles: DebouncedInputHandles): void {
  if (handles.timer !== null) clearTimeout(handles.timer);
  handles.timer = null;
  handles.commit = null;
}

function invokeCommit(commit: (value: string) => void, value: string): void {
  try {
    commit(value);
  } catch (error) {
    console.error("Failed to commit a debounced input value", error);
  }
}

export const debouncedInputLogic = createStoreLogic<
  DebouncedInputState,
  DebouncedInputEvents,
  never,
  { value: string }
>({
  context: input => ({
    committedValue: input.value,
    draftValue: input.value,
    externalValue: input.value,
    generation: 0,
    phase: "idle",
  }),
  on: {
    externalValueObserved: (context, event, enqueue) => {
      if (event.value === context.externalValue) return;
      enqueue.effect(({ trigger }) => clearPending(handlesFor(trigger)));
      return {
        committedValue: event.value,
        draftValue: event.value,
        externalValue: event.value,
        generation: context.generation + 1,
        phase: "idle",
      };
    },
    reset: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => clearPending(handlesFor(trigger)));
      return {
        committedValue: event.value,
        draftValue: event.value,
        externalValue: event.value,
        generation: context.generation + 1,
        phase: "idle",
      };
    },
    valueChanged: (context, event, enqueue) => {
      const generation = context.generation + 1;
      enqueue.effect(({ trigger }) => {
        const handles = handlesFor(trigger);
        clearPending(handles);
        if (event.value === context.committedValue) return;
        handles.commit = event.commit;
        handles.timer = setTimeout(
          () => trigger.commitElapsed({ generation }),
          Math.max(0, event.delayMs)
        );
      });
      return {
        ...context,
        draftValue: event.value,
        generation,
        phase: event.value === context.committedValue ? "idle" : "waiting",
      };
    },
    commitElapsed: (context, event, enqueue) => {
      if (event.generation !== context.generation || context.phase !== "waiting") return;
      enqueue.effect(({ trigger }) => {
        const handles = handlesFor(trigger);
        const commit = handles.commit;
        clearPending(handles);
        if (commit) invokeCommit(commit, context.draftValue);
      });
      return {
        ...context,
        committedValue: context.draftValue,
        externalValue: context.draftValue,
        phase: "idle",
      };
    },
    cleared: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        clearPending(handlesFor(trigger));
        invokeCommit(event.commit, "");
      });
      return {
        ...context,
        committedValue: "",
        draftValue: "",
        externalValue: "",
        generation: context.generation + 1,
        phase: "idle",
      };
    },
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        const handles = handlesByTrigger.get(trigger);
        if (handles) clearPending(handles);
        handlesByTrigger.delete(trigger);
      });
      return { ...context, generation: context.generation + 1, phase: "idle" };
    },
  },
});
