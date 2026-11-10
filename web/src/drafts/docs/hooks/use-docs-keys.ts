import { useEffect, type Dispatch, type SetStateAction } from "react";

import { withSearch, withoutOverlays, type DocsView } from "../view";

/** Cmd/Ctrl+K opens search from anywhere; Escape closes whatever overlay is open. */
export function useDocsKeys(update: Dispatch<SetStateAction<DocsView>>) {
  useEffect(() => {
    const handle = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        update(withSearch);
      } else if (event.key === "Escape") {
        update(withoutOverlays);
      }
    };
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, [update]);
}
