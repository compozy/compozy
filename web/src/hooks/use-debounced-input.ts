import { useEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { debouncedInputLogic } from "./debounced-input-store";

interface UseDebouncedInputOptions {
  delayMs?: number;
  externalValue: string;
  onCommit: (value: string) => void;
}

/** Debounced interaction state with an explicit draft/committed split. */
export function useDebouncedInput({
  delayMs = 180,
  externalValue,
  onCommit,
}: UseDebouncedInputOptions) {
  const store = useStore(debouncedInputLogic, { value: externalValue });
  const draftValue = useSelector(store, snapshot => snapshot.context.draftValue);
  const committedValue = useSelector(store, snapshot => snapshot.context.committedValue);

  useEffect(() => {
    store.trigger.externalValueObserved({ value: externalValue });
  }, [externalValue, store]);

  useEffect(() => () => store.trigger.disposed(), [store]);

  return {
    committedValue,
    draftValue,
    clear: () => store.trigger.cleared({ commit: onCommit }),
    reset: (value = externalValue) => store.trigger.reset({ value }),
    setDraftValue: (value: string) =>
      store.trigger.valueChanged({ commit: onCommit, delayMs, value }),
  };
}
