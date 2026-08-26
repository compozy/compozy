import {
  destroyTerminalInstance,
  TerminalWriteAbandonedError,
  type TerminalViewHandle,
} from "@compozy/ui";
import { useEffect, useRef, useState } from "react";

/** Owns one read-only terminal emulator's retained output lifecycle. */
export function useTerminalReplay(instanceId: string, output: string, enabled = true) {
  const handleRef = useRef<TerminalViewHandle>(null);
  // Attachment is per emulator identity: when `instanceId` changes the old
  // buffer is destroyed and a new one mounts, so a bare boolean would stay
  // stale-true and the write below would never re-run for the new buffer.
  const [attachedFor, setAttachedFor] = useState<string | null>(null);
  const attached = attachedFor === instanceId;

  useEffect(() => () => destroyTerminalInstance(instanceId), [instanceId]);
  useEffect(() => {
    if (!attached || !enabled) return;
    handleRef.current?.reset();
    void handleRef.current?.write(output).catch(cause => {
      if (cause instanceof TerminalWriteAbandonedError) return;
      throw cause;
    });
  }, [attached, enabled, output]);

  return { handleRef, onAttached: () => setAttachedFor(instanceId) };
}
