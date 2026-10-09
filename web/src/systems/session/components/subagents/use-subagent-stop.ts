import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

import type { SubagentView } from "./types";

type StoppedRows = ReadonlyMap<string, SubagentView["status"]>;

/**
 * Pending state for one Stop control over `rows`. The daemon answers a stop
 * with 202 and settles later, so the control stays `Stopping…` while any row it
 * stopped is still in the state it had at the click; rows that appear after
 * the click (a new subagent starting) do not end it. Failure raises
 * `failureCopy` as a toast and brings the control back. Nothing is set after
 * the control unmounts.
 */
export function useSubagentStop(
  onStop: (() => Promise<unknown>) | undefined,
  failureCopy: string,
  rows: readonly SubagentView[]
): { pending: boolean; stop: () => void } {
  const [stopped, setStopped] = useState<StoppedRows | null>(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const pending = stopped !== null && rows.some(row => stopped.get(row.id) === row.status);

  const stop = () => {
    if (!onStop || pending) return;
    setStopped(new Map(rows.map(row => [row.id, row.status])));
    onStop().catch(error => {
      console.error(failureCopy, error);
      toast.error(failureCopy);
      if (mounted.current) setStopped(null);
    });
  };

  return { pending, stop };
}
