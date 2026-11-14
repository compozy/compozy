import type { ReactNode } from "react";

import { useSessionSubagentActions } from "../../hooks/use-session-subagent-actions";
import { SessionSubagentActionsContext } from "../../contexts/session-subagent-actions-context-value";
import { SubagentDeleteDialog } from "../subagents/subagent-delete-dialog";

/**
 * The sessions list's one subagent-verb owner, above every thread: the branch
 * whose row opened a confirmation can leave the list (the stream drops the
 * parent's summary before the DELETE answers) without taking the dialog with
 * it. Branches read the verbs from the context and supply each row's scope.
 */
export function SessionListSubagentActionsHost({ children }: { children: ReactNode }) {
  const actions = useSessionSubagentActions();
  return (
    <SessionSubagentActionsContext value={actions}>
      {children}
      <SubagentDeleteDialog {...actions.dialog} />
    </SessionSubagentActionsContext>
  );
}
