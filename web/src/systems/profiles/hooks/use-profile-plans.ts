import { useQuery } from "@tanstack/react-query";

import { archivePlanOptions, deletePlanOptions, renamePlanOptions } from "../lib/query-options";

/**
 * Plans are read from the daemon, never recomputed here.
 *
 * Each plan carries the revision its mutation must quote, so what the dialog
 * shows and what the daemon executes are the same decision — a client-side scan
 * would be a second, drifting answer.
 */
export function useRenamePlan(name: string, newName: string, enabled = true) {
  return useQuery(renamePlanOptions(name, newName, enabled));
}

export function useArchivePlan(name: string, enabled = true) {
  return useQuery(archivePlanOptions(name, enabled));
}

export function useDeletePlan(name: string, enabled = true) {
  return useQuery(deletePlanOptions(name, enabled));
}
