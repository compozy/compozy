import { createStoreLogic } from "@xstate/store";

const BLUR_DISMISS_DELAY_MS = 120;

interface ReferenceAutocompleteState {
  activeIndex: number;
  pendingCaret: number | null;
  query: string | null;
}

type ReferenceAutocompleteEvents = {
  activeIndexChanged: { index: number };
  blurred: Record<never, never>;
  caretApplied: { caret: number };
  dismissed: Record<never, never>;
  dismissElapsed: Record<never, never>;
  disposed: Record<never, never>;
  refreshed: { query: string | null };
  selected: { caret: number };
};

interface AutocompleteHandles {
  blurTimer: ReturnType<typeof setTimeout> | null;
}

interface AutocompleteTrigger {
  dismissElapsed: () => void;
}

const handlesByTrigger = new WeakMap<object, AutocompleteHandles>();

function handlesFor(trigger: object): AutocompleteHandles {
  const existing = handlesByTrigger.get(trigger);
  if (existing) return existing;
  const created = { blurTimer: null };
  handlesByTrigger.set(trigger, created);
  return created;
}

function cancelBlur(trigger: object): void {
  const handles = handlesByTrigger.get(trigger);
  if (!handles || handles.blurTimer === null) return;
  clearTimeout(handles.blurTimer);
  handles.blurTimer = null;
}

export const referenceAutocompleteLogic = createStoreLogic<
  ReferenceAutocompleteState,
  ReferenceAutocompleteEvents
>({
  context: { activeIndex: 0, pendingCaret: null, query: null },
  on: {
    refreshed: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => cancelBlur(trigger));
      return { ...context, activeIndex: 0, query: event.query };
    },
    activeIndexChanged: (context, event) => ({ ...context, activeIndex: event.index }),
    selected: (context, event, enqueue) => {
      enqueue.effect(({ trigger }) => cancelBlur(trigger));
      return { ...context, pendingCaret: event.caret, query: null };
    },
    caretApplied: (context, event) =>
      context.pendingCaret === event.caret ? { ...context, pendingCaret: null } : undefined,
    dismissed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => cancelBlur(trigger));
      return { ...context, query: null };
    },
    blurred: (context, _event, enqueue) => {
      if (context.query === null) return;
      enqueue.effect(({ trigger }) => {
        cancelBlur(trigger);
        const handles = handlesFor(trigger);
        handles.blurTimer = setTimeout(
          () => (trigger as AutocompleteTrigger).dismissElapsed(),
          BLUR_DISMISS_DELAY_MS
        );
      });
      return context;
    },
    dismissElapsed: context => ({ ...context, query: null }),
    disposed: (context, _event, enqueue) => {
      enqueue.effect(({ trigger }) => {
        cancelBlur(trigger);
        handlesByTrigger.delete(trigger);
      });
      return { ...context, pendingCaret: null, query: null };
    },
  },
});
