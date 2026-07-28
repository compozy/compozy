import { useSessions } from "@/systems/session";
import { useTaskDashboard } from "@/systems/tasks";

import type { HomeScope } from "../lib/home-scope";
import { homeWorkingNowSessionFilters } from "../lib/home-working-now-query";
import type { HomeRunCardModel, HomeWorkingNowModel, HomeWorkingNowStatus } from "../types";
import { elapsedSecondsSince } from "./use-elapsed-ticker";

/** Reads sessions and task runs only after workspace scope has settled. */
export function useHomeWorkingNow(scope: HomeScope, enabled: boolean): HomeWorkingNowModel {
  const sessionsQuery = useSessions(scope.workspaceParam || null, {
    enabled,
    filters: homeWorkingNowSessionFilters(),
  });
  const dashboardQuery = useTaskDashboard(
    {
      scope: scope.taskScope.scope,
      workspace: scope.taskScope.workspace,
    },
    { enabled }
  );

  const sessionsAtMs = sessionsQuery.dataUpdatedAt;
  const dashboardAtMs = dashboardQuery.dataUpdatedAt;
  const sessionsAtSeconds = Math.floor(sessionsAtMs / 1000);
  const cards: HomeRunCardModel[] = [];

  const sessions = sessionsQuery.data ?? [];
  const sessionIds = new Set(sessions.map(session => session.id));
  for (const session of sessions) {
    const activity = session.activity;
    cards.push({
      key: `session:${session.id}`,
      kind: "session",
      agentName: session.agent_name,
      title: session.name?.trim() ? session.name : session.agent_name,
      subtitle: activity?.current_tool?.trim()
        ? `Running ${activity.current_tool}`
        : "Working on the current turn",
      elapsedBaseSeconds: elapsedSecondsSince(
        sessionsAtSeconds,
        activity?.turn_started_at ?? session.created_at ?? ""
      ),
      baseAtMs: sessionsAtMs,
      sessionLink: { agentName: session.agent_name, sessionId: session.id },
    });
  }

  const activeRuns = dashboardQuery.data?.active_runs;
  const runItems = activeRuns?.items ?? [];
  // A run bound to a loaded session is already represented by that session card.
  const uniqueRunItems = runItems.filter(run => !run.session_id || !sessionIds.has(run.session_id));
  for (const run of uniqueRunItems) {
    cards.push({
      key: `run:${run.run_id}`,
      kind: "task_run",
      agentName: run.task_owner?.ref ?? "",
      title: run.task_title,
      subtitle: `Run ${run.run_status}`,
      elapsedBaseSeconds: Math.max(0, Math.floor(run.age_ms / 1000)),
      baseAtMs: dashboardAtMs,
      runLink: { taskId: run.task_id, runId: run.run_id },
    });
  }

  // Count the loaded, deduplicated cards rather than unverifiable paged totals.
  const sessionCount = sessions.length;
  const runCount = uniqueRunItems.length;
  // Either source can make the combined surface incomplete.
  const anyLoading = sessionsQuery.isLoading || dashboardQuery.isLoading;
  const anyError = sessionsQuery.isError || dashboardQuery.isError;
  const errorMessage = anyError
    ? firstErrorMessage(sessionsQuery.error, dashboardQuery.error)
    : null;

  return {
    cards,
    total: cards.length,
    sessionCount,
    runCount,
    status: deriveWorkingNowStatus(cards.length > 0, anyLoading, anyError),
    errorMessage,
  };
}

/** Keeps loaded cards visible as partial and avoids flashing an error while empty reads settle. */
function deriveWorkingNowStatus(
  hasCards: boolean,
  anyLoading: boolean,
  anyError: boolean
): HomeWorkingNowStatus {
  if (hasCards) {
    return anyError ? "partial" : "ready";
  }
  if (anyLoading) {
    return "loading";
  }
  return anyError ? "error" : "ready";
}

function firstErrorMessage(...errors: unknown[]): string | null {
  for (const error of errors) {
    if (error instanceof Error && error.message.trim()) {
      return error.message;
    }
  }
  return null;
}
