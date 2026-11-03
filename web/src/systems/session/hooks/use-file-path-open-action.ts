import { useRef, useState, type MouseEvent } from "react";

import type { FilePathToken } from "../lib/file-path-links";
import type { OpenInEditor } from "./use-open-in-editor";

/** One open request at a time per path span; `pending` holds while the shell answers. */
export function useFilePathOpenAction(opener: OpenInEditor, token: FilePathToken) {
  const inFlight = useRef<number | null>(null);
  const nextRequest = useRef(0);
  const [pendingRequest, setPendingRequest] = useState<number | null>(null);
  const run = async () => {
    if (inFlight.current !== null) return;
    nextRequest.current += 1;
    const request = nextRequest.current;
    inFlight.current = request;
    setPendingRequest(request);
    try {
      await opener.open(token);
    } finally {
      inFlight.current = null;
      // Only the request that set the pending state clears it.
      setPendingRequest(current => (current === request ? null : current));
    }
  };
  // Plain clicks keep their meaning (selection, nothing else); only Cmd/Ctrl+click opens.
  const onModifierClick = (event: MouseEvent) => {
    const modified = opener.modifier === "meta" ? event.metaKey : event.ctrlKey;
    if (!modified) return;
    event.preventDefault();
    event.stopPropagation();
    void run();
  };
  return { pending: pendingRequest !== null, run, onModifierClick };
}
