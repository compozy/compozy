import { useContext } from "react";

import { SessionThreadLiveDataContext } from "../session-thread-live-data-context";

/** `false` while the thread's window is paused: motion stops, words carry the state. */
export function useSessionThreadLiveData(): boolean {
  return useContext(SessionThreadLiveDataContext);
}
