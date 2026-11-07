import { useRef } from "react";

import type { FilePathToken } from "../lib/file-path-links";
import type { OpenInEditor } from "./use-open-in-editor";

/** One editor open at a time per path span, while the desktop shell answers. */
export function useFilePathEditorOpen(editor: OpenInEditor | null, token: FilePathToken) {
  const inFlight = useRef(false);
  return () => {
    if (!editor || inFlight.current) return;
    inFlight.current = true;
    void editor.open(token).finally(() => {
      inFlight.current = false;
    });
  };
}
