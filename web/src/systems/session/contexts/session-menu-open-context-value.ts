import { createContext } from "react";

import type { SessionListTarget } from "../types";

/**
 * Provided by the session list and the Sessions board: opens another session the way their rows
 * and cards do. A row's menu uses it for a rolled-up linked-work item's child session. Absent
 * (null) where no surface can open one, and the entry hides.
 */
export const SessionMenuOpenContext = createContext<((target: SessionListTarget) => void) | null>(
  null
);
