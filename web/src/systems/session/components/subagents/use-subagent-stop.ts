import { useState } from "react";
import { toast } from "sonner";

/**
 * Pending state for one Stop control: `Stopping…` while the request is in
 * flight, then the button returns. Success needs no copy (the roster reports
 * the settle); failure raises `failureCopy` as a toast.
 */
export function useSubagentStop(
  onStop: (() => Promise<unknown>) | undefined,
  failureCopy: string
): { pending: boolean; stop: () => void } {
  const [pending, setPending] = useState(false);

  const stop = () => {
    if (!onStop || pending) return;
    setPending(true);
    onStop().then(
      () => setPending(false),
      error => {
        console.error(failureCopy, error);
        toast.error(failureCopy);
        setPending(false);
      }
    );
  };

  return { pending, stop };
}
