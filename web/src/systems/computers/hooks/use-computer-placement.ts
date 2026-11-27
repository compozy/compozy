import { useContext } from "react";
import { QueryClient, QueryClientContext, useQuery } from "@tanstack/react-query";

import { LOCAL_COMPUTER_ID } from "@/lib/computer-scope";

import { ComputersApiError } from "../adapters/computers-api";
import type { AutoResolutionSummary } from "../components/run-on-picker";
import { computerPlacementOptions } from "../lib/query-options";
import type { ComputerPlacementResponse } from "../types";

const NOOP_QUERY_CLIENT = new QueryClient({
  defaultOptions: { queries: { enabled: false, retry: false } },
});

export interface ComputerPlacementState {
  /** True while `POST /api/computers/placement` is resolving the draft workspace. */
  isLoading: boolean;
  /** Resolved winner for `Auto → <computer>`, or `null` when unavailable/ineligible. */
  resolution: AutoResolutionSummary | null;
  /** Raw placement payload (carries the target computer's matching `workspace_id`). */
  placement: ComputerPlacementResponse | null;
  /** True when the daemon explicitly reported `no_eligible_computer` (or no workspace can run). */
  noEligible: boolean;
  /** Per-computer exclusion reasons from `ErrorPayload.details`, when reported. */
  reasons: Record<string, string> | undefined;
}

/**
 * Calls `POST /api/computers/placement` to preview which computer `Auto` will select
 * for the active draft workspace (`_uiux.md` U4, US-015, UT-106).
 */
export function useComputerPlacement(input: {
  enabled: boolean;
  workspace: string | null | undefined;
  projectComputerId?: string;
}): ComputerPlacementState {
  const contextClient = useContext(QueryClientContext);
  const queryClient = contextClient ?? NOOP_QUERY_CLIENT;
  const workspace = input.workspace?.trim() ?? "";
  const active = input.enabled && contextClient !== undefined && workspace !== "";

  const query = useQuery(
    computerPlacementOptions({ workspace, projectComputerId: input.projectComputerId }, active),
    queryClient
  );

  if (!input.enabled) {
    return {
      isLoading: false,
      resolution: null,
      placement: null,
      noEligible: false,
      reasons: undefined,
    };
  }

  if (workspace === "") {
    return {
      isLoading: false,
      resolution: null,
      placement: null,
      noEligible: true,
      reasons: undefined,
    };
  }

  if (query.isPending) {
    return {
      isLoading: true,
      resolution: null,
      placement: null,
      noEligible: false,
      reasons: undefined,
    };
  }

  if (query.data) {
    const computerName =
      query.data.computer_id === LOCAL_COMPUTER_ID ? "This computer" : query.data.name;
    return {
      isLoading: false,
      resolution: {
        computerId: query.data.computer_id,
        computerName,
        reason: "Picks the least busy computer",
      },
      placement: query.data,
      noEligible: false,
      reasons: undefined,
    };
  }

  const apiError = query.error instanceof ComputersApiError ? query.error : null;
  const noEligible =
    apiError?.code === "no_eligible_computer" || apiError?.status === 409 || query.isError;

  return {
    isLoading: false,
    resolution: null,
    placement: null,
    noEligible,
    reasons: apiError?.details,
  };
}
