import { useEffect, useRef } from "react";

/** Brings a row into its list's own scroller; the sliding track must not scroll sideways. */
function revealInList(target: HTMLElement) {
  const list = target.closest<HTMLElement>('[data-slot="subagent-roster-list"]');
  if (!list) return;
  const box = list.getBoundingClientRect();
  const row = target.getBoundingClientRect();
  if (row.top < box.top) list.scrollTop -= box.top - row.top;
  else if (row.bottom > box.bottom) list.scrollTop += row.bottom - box.bottom;
}

/** Focus follows the panel: in to Back, and out to the row that opened the subagent. */
export function useSubagentPanelFocus(shownId: string | null) {
  const track = useRef<HTMLDivElement>(null);
  const previous = useRef(shownId);
  useEffect(() => {
    const was = previous.current;
    previous.current = shownId;
    if (was === shownId) return;
    const node = track.current;
    const selector = shownId ? "[data-subagent-back]" : `[data-subagent-id="${was}"] button`;
    // The row can be gone by the time the reader comes back; the panel keeps focus either way.
    const target = node?.querySelector<HTMLElement>(selector) ?? node?.querySelector("button");
    if (!target) return;
    target.focus({ preventScroll: true });
    revealInList(target);
  }, [shownId]);
  return track;
}
