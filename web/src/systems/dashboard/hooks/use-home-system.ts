import { useQuery } from "@tanstack/react-query";

import { buildHomeSystemModel, type HomeSystemModel } from "../lib/home-system";
import { statusOptions } from "@/systems/status";

export function useHomeSystem(
  hookRunsToday: number | undefined,
  hookFailuresToday: number | undefined,
  retentionDays: number | undefined
): HomeSystemModel {
  const statusQuery = useQuery(statusOptions());
  return buildHomeSystemModel(statusQuery.data, {
    hookRunsToday,
    hookFailuresToday,
    retentionDays,
  });
}
