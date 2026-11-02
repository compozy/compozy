import { useEffect, useRef } from "react";

/** Focus follows the panel: in to Back, and out to the row that opened the subagent. */
export function useSubagentPanelFocus(shownId: string | null) {
  const track = useRef<HTMLDivElement>(null);
  const previous = useRef(shownId);
  useEffect(() => {
    const was = previous.current;
    previous.current = shownId;
    if (was === shownId) return;
    const selector = shownId ? "[data-subagent-back]" : `[data-subagent-id="${was}"] button`;
    track.current?.querySelector<HTMLElement>(selector)?.focus({ preventScroll: true });
  }, [shownId]);
  return track;
}
