import { useQuery } from "@tanstack/react-query";
import { useState } from "react";

import { useProfileReadScope } from "@/systems/profiles";

import {
  ACTIVITY_ENTRIES_LIMIT,
  deriveActivityModel,
  type ActivityFilter,
  type ActivityModel,
  type ActivityRow,
} from "../lib/activity-view";
import { permissionEntriesOptions, permissionRollupOptions } from "../lib/query-options";

export interface ActivityPageState {
  filter: ActivityFilter;
  setFilter: (filter: ActivityFilter) => void;
  query: string;
  setQuery: (query: string) => void;
  model: ActivityModel | null;
  rows: readonly ActivityRow[];
  truncated: boolean;
  isLoading: boolean;
  error: Error | null;
}

/** Totals come from the rollups, whose population is complete; the log is its newest rows. */
export function useActivityPage(): ActivityPageState {
  const [filter, setFilter] = useState<ActivityFilter>("unattended");
  const [query, setQuery] = useState("");
  const { params } = useProfileReadScope();
  const rollup = useQuery(permissionRollupOptions(filter, params));
  const entries = useQuery(permissionEntriesOptions(filter, params));
  const [now] = useState(() => new Date());
  const model =
    rollup.data && entries.data
      ? deriveActivityModel(
          filter,
          rollup.data.rollups,
          entries.data,
          new Date(rollup.data.until),
          now
        )
      : null;
  const needle = query.trim().toLowerCase();
  const rows = (model?.rows ?? []).filter(row =>
    `${row.action} ${row.detail} ${row.agentName}`.toLowerCase().includes(needle)
  );
  return {
    filter,
    setFilter,
    query,
    setQuery,
    model,
    rows,
    truncated: (entries.data?.length ?? 0) >= ACTIVITY_ENTRIES_LIMIT,
    isLoading: rollup.isPending || entries.isPending,
    error: rollup.error ?? entries.error,
  };
}
