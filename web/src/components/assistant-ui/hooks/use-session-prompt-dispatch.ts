import { useContext, useSyncExternalStore } from "react";

import { SessionPromptDispatchContext } from "../session-prompt-dispatch-store";

export function useSessionPromptDispatch() {
  const store = useContext(SessionPromptDispatchContext);
  const snapshot = useSyncExternalStore(store.subscribe, store.getSnapshot, store.getSnapshot);
  return { cancelPending: store.cancelPending, ...snapshot };
}
