import { use } from "react";
import { useSelector } from "@xstate/store-react";

import { SessionPromptDispatchContext } from "../session-prompt-dispatch-store";

export function useSessionPromptDispatch() {
  const store = use(SessionPromptDispatchContext);
  if (store === null) {
    throw new Error("useSessionPromptDispatch requires SessionPromptDispatchPendingProvider");
  }
  const canceled = useSelector(store, snapshot => snapshot.context.canceled);
  const pending = useSelector(store, snapshot => snapshot.context.pending);
  return {
    cancelPending: () => store.trigger.pendingCanceled({}),
    canceled,
    pending,
  };
}
