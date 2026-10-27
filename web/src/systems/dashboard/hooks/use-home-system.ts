import { useQuery } from "@tanstack/react-query";

import { buildHomeSystemModel, type HomeSystemModel } from "../lib/home-system";
import { statusOptions } from "@/systems/status";

export interface HomeSystemState {
  system: HomeSystemModel;
  /** Daemon status has no answer yet: the model cannot call the system normal or not. */
  loading: boolean;
}

export function useHomeSystem(
  hookRunsToday: number | undefined,
  hookFailuresToday: number | undefined,
  retentionDays: number | undefined
): HomeSystemState {
  const statusQuery = useQuery(statusOptions());
  return {
    system: buildHomeSystemModel(statusQuery.data, {
      hookRunsToday,
      hookFailuresToday,
      retentionDays,
    }),
    loading: statusQuery.isLoading,
  };
}
