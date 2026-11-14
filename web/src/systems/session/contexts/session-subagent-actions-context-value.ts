import { createContext } from "react";

import type { SessionSubagentActionsModel } from "../hooks/use-session-subagent-actions";

/**
 * The rail's subagent verbs, provided once by the session list above its
 * threads so a confirmation survives the branch — and the dialog host — whose
 * row opened it. A branch supplies each row's scope. Absent (null) where no
 * stable host owns the dialog: the row's menu then hides Stop and Delete.
 */
export type SessionSubagentActions = Pick<SessionSubagentActionsModel, "openDelete" | "stop">;

export const SessionSubagentActionsContext = createContext<SessionSubagentActions | null>(null);
