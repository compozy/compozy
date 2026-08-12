import type {
  LoopEditorEvents,
  LoopEditorState,
  LoopEditorEnqueue,
} from "./loop-editor-store-contracts";

const AUTO_VALIDATE_DEBOUNCE_MS = 400;

interface LoopEditorLifecycleHandles {
  execute: (() => void) | null;
  timer: ReturnType<typeof setTimeout> | null;
}

const handlesByTrigger = new WeakMap<object, LoopEditorLifecycleHandles>();

function handlesFor(trigger: object): LoopEditorLifecycleHandles {
  const existing = handlesByTrigger.get(trigger);
  if (existing) return existing;
  const created = { execute: null, timer: null };
  handlesByTrigger.set(trigger, created);
  return created;
}

function clearLifecycleTimer(handles: LoopEditorLifecycleHandles): void {
  if (handles.timer !== null) clearTimeout(handles.timer);
  handles.timer = null;
}

function cancelAutomaticValidation(enqueue: LoopEditorEnqueue): void {
  enqueue.effect(({ trigger }) => {
    const handles = handlesByTrigger.get(trigger);
    if (!handles) return;
    clearLifecycleTimer(handles);
    handles.execute = null;
  });
}

export const loopEditorLifecycleTransitions = {
  annotationsStatusObserved: (
    current: LoopEditorState,
    event: LoopEditorEvents["annotationsStatusObserved"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (current.annotationsUnavailable === event.failed) return;
    if (event.failed) enqueue.effect(() => event.notify());
    return { ...current, annotationsUnavailable: event.failed };
  },
  automaticValidationCancelled: (
    current: LoopEditorState,
    _event: LoopEditorEvents["automaticValidationCancelled"],
    enqueue: LoopEditorEnqueue
  ) => {
    cancelAutomaticValidation(enqueue);
    return { ...current, automaticValidationRevision: null };
  },
  automaticValidationRequested: (
    current: LoopEditorState,
    event: LoopEditorEvents["automaticValidationRequested"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (!event.enabled) {
      cancelAutomaticValidation(enqueue);
      return { ...current, automaticValidationRevision: null };
    }
    if (event.revision !== current.structuralRevision) return;
    enqueue.effect(({ trigger }) => {
      const handles = handlesFor(trigger);
      clearLifecycleTimer(handles);
      handles.execute = event.execute;
      handles.timer = setTimeout(
        () => trigger.automaticValidationElapsed({ revision: event.revision }),
        AUTO_VALIDATE_DEBOUNCE_MS
      );
    });
    return { ...current, automaticValidationRevision: event.revision };
  },
  automaticValidationElapsed: (
    current: LoopEditorState,
    event: LoopEditorEvents["automaticValidationElapsed"],
    enqueue: LoopEditorEnqueue
  ) => {
    if (
      current.automaticValidationRevision !== event.revision ||
      current.structuralRevision !== event.revision
    ) {
      return;
    }
    enqueue.effect(({ trigger }) => {
      const handles = handlesFor(trigger);
      clearLifecycleTimer(handles);
      const execute = handles.execute;
      handles.execute = null;
      execute?.();
    });
    return { ...current, automaticValidationRevision: null };
  },
  lifecycleDisposed: (
    current: LoopEditorState,
    _event: LoopEditorEvents["lifecycleDisposed"],
    enqueue: LoopEditorEnqueue
  ) => {
    enqueue.effect(({ trigger }) => {
      const handles = handlesByTrigger.get(trigger);
      if (handles) clearLifecycleTimer(handles);
      handlesByTrigger.delete(trigger);
    });
    return { ...current, automaticValidationRevision: null };
  },
};
