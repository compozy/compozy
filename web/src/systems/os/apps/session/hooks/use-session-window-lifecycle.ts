import { useContext } from "react";

import {
  SessionWindowLifecycleContext,
  type SessionWindowLifecycle,
} from "../session-window-lifecycle-context";

/** The window's shared lifecycle; `null` only outside a `SessionWindowLifecycleHost`. */
export function useSessionWindowLifecycle(): SessionWindowLifecycle | null {
  return useContext(SessionWindowLifecycleContext);
}
