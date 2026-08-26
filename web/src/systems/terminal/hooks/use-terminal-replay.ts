import {
  destroyTerminalInstance,
  TerminalWriteAbandonedError,
  type TerminalViewHandle,
} from "@compozy/ui";
import { useEffect, useRef, useState } from "react";

/** Owns one read-only terminal emulator's retained output lifecycle. */
export function useTerminalReplay(instanceId: string, output: string, enabled = true) {
  const handleRef = useRef<TerminalViewHandle>(null);
  const [attached, setAttached] = useState(false);

  useEffect(() => () => destroyTerminalInstance(instanceId), [instanceId]);
  useEffect(() => {
    if (!attached || !enabled) return;
    handleRef.current?.reset();
    void handleRef.current?.write(output).catch(cause => {
      if (cause instanceof TerminalWriteAbandonedError) return;
      throw cause;
    });
  }, [attached, enabled, output]);

  return { handleRef, onAttached: () => setAttached(true) };
}
