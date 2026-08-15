import type { AutomationRun } from "../types";

export type AutomationRunDestination =
  | { kind: "loop-run"; id: string; workspaceId?: string }
  | { kind: "session"; id: string };

/** Shared loop-first destination precedence for every automation run surface. */
export function automationRunDestination(
  run: AutomationRun,
  loopWorkspaceId?: string
): AutomationRunDestination | null {
  if (run.loop_run_id) {
    return {
      kind: "loop-run",
      id: run.loop_run_id,
      ...(loopWorkspaceId ? { workspaceId: loopWorkspaceId } : {}),
    };
  }
  if (run.session_id) return { kind: "session", id: run.session_id };
  return null;
}
