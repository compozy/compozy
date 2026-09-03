import { useState } from "react";

import type { TerminalSelectionRange, TerminalViewHandle } from "@compozy/ui";

export function useTerminalSelection(
  handleRef: React.RefObject<TerminalViewHandle | null>,
  onSelectionChange?: (selection: TerminalSelectionRange | null) => void
) {
  const [selection, setSelection] = useState<TerminalSelectionRange | null>(null);
  return {
    selection,
    readSelection: () => {
      const range = handleRef.current?.getSelectionRange() ?? null;
      const next = range && range.text.trim() !== "" ? range : null;
      setSelection(next);
      onSelectionChange?.(next);
    },
  };
}
