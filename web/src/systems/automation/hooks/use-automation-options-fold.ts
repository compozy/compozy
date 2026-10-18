import { useEffect, useRef, useState } from "react";

import type { AutomationEditorSection } from "../lib/automation-form-draft";

/**
 * The Options fold, held above the preview swap so a fold someone opened
 * survives a look at the preview; a deep link to Options opens and scrolls it.
 */
export function useAutomationOptionsFold(
  defaultOpen: boolean,
  section: AutomationEditorSection | undefined
) {
  const atOptions = section === "options";
  const [open, setOpen] = useState(defaultOpen || atOptions);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (atOptions) ref.current?.scrollIntoView?.({ block: "nearest" });
  }, [atOptions]);
  return { open, ref, setOpen };
}
