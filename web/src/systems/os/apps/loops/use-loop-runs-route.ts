import { useState } from "react";
import { useNavigate } from "@tanstack/react-router";

import { useLoopRuns, type LoopOutcomeValue } from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

export interface LoopRunsRouteSearch {
  origin?: "catalog" | "session";
  origin_session?: string;
}

export function useLoopRunsRoute(search: LoopRunsRouteSearch) {
  const { activeWorkspace, activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  const workspaceLabel = activeWorkspace?.name ?? activeWorkspace?.id ?? "workspace";
  const [outcome, setOutcome] = useState<LoopOutcomeValue>("all");
  const navigate = useNavigate({ from: "/loop-runs" });
  const runsQuery = useLoopRuns(
    workspaceId,
    { origin: search.origin, origin_session: search.origin_session },
    workspaceId !== ""
  );

  const setOrigin = (origin: LoopRunsRouteSearch["origin"]) => {
    void navigate({
      to: "/loop-runs",
      search: current => ({
        ...current,
        origin,
        origin_session: origin === "session" ? current.origin_session : undefined,
      }),
    });
  };

  const setOriginSession = (originSession: string) => {
    void navigate({
      to: "/loop-runs",
      search: current => ({
        ...current,
        origin: "session",
        origin_session: originSession || undefined,
      }),
    });
  };

  return {
    outcome,
    runsQuery,
    setOrigin,
    setOriginSession,
    setOutcome,
    workspaceId,
    workspaceLabel,
  };
}

export function validateLoopRunsSearch(search: Record<string, unknown>): LoopRunsRouteSearch {
  const origin =
    search.origin === "catalog" || search.origin === "session" ? search.origin : undefined;
  const originSession =
    typeof search.origin_session === "string" ? search.origin_session.trim() : "";
  return { origin, origin_session: originSession || undefined };
}
