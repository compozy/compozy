import { useState } from "react";

/** ↑/↓ in the Search query walk the root's last queries, newest first (US-014.AC-7). */
export function useSearchHistoryCursor(
  history: readonly string[],
  setDraft: (next: string) => void
) {
  const [cursor, setCursor] = useState(-1);
  return {
    reset: () => setCursor(-1),
    step: (delta: number) => {
      const next = Math.min(Math.max(cursor + delta, -1), history.length - 1);
      setCursor(next);
      setDraft(next < 0 ? "" : (history[next] ?? ""));
    },
  };
}
