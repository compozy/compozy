import { useEffect, useRef, useState } from "react";
import { toast } from "sonner";

/**
 * Pending state for one Stop control. The daemon answers a stop with 202 and
 * settles the subagent later, so the control stays `Stopping…` until the
 * caller's `settleKey` (the stopped rows' state) changes, never just because
 * the request returned. Failure raises `failureCopy` as a toast and brings the
 * control back. Nothing is set after the control unmounts.
 */
export function useSubagentStop(
  onStop: (() => Promise<unknown>) | undefined,
  failureCopy: string,
  settleKey: string
): { pending: boolean; stop: () => void } {
  const [stoppedAt, setStoppedAt] = useState<string | null>(null);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const pending = stoppedAt !== null && stoppedAt === settleKey;

  const stop = () => {
    if (!onStop || pending) return;
    setStoppedAt(settleKey);
    onStop().catch(error => {
      console.error(failureCopy, error);
      toast.error(failureCopy);
      if (mounted.current) setStoppedAt(null);
    });
  };

  return { pending, stop };
}
