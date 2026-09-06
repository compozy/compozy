import { type RefObject, useEffect, useId } from "react";

import { collectFindRanges } from "../session-find-ranges";

/*
 * In-transcript match marks (S8) through the CSS Custom Highlight API: every
 * occurrence of the committed query inside the rendered rows is painted as
 * `::highlight(session-find-match)`, the first occurrence in the active match's
 * row as `::highlight(session-find-active)` (styles in `src/styles.css`).
 * Ranges are cut with the daemon's per-code-point folding, in the original
 * text, so Unicode case pairs (İ, final sigma) never shift an offset.
 *
 * `CSS.highlights` is one registry for the whole document, so the two names are
 * shared: every mounted viewport contributes its ranges and the union is what
 * gets registered. A viewport closing removes only its own contribution.
 * Ranges never touch React's DOM; a mutation observer repaints once per frame
 * while rows stream or mount, and the marks drop with the query. Browsers
 * without the API show the list and the row landing only.
 */

export const FIND_MATCH_HIGHLIGHT = "session-find-match";
export const FIND_ACTIVE_HIGHLIGHT = "session-find-active";

interface HostRanges {
  matches: Range[];
  active: Range | null;
}

const hostRanges = new Map<string, HostRanges>();

function highlightRegistry(): HighlightRegistry | null {
  if (typeof CSS === "undefined" || !("highlights" in CSS) || typeof Highlight !== "function") {
    return null;
  }
  return CSS.highlights;
}

function paintUnion(registry: HighlightRegistry): void {
  const matches: Range[] = [];
  const actives: Range[] = [];
  for (const host of hostRanges.values()) {
    matches.push(...host.matches);
    if (host.active) actives.push(host.active);
  }
  if (matches.length === 0) registry.delete(FIND_MATCH_HIGHLIGHT);
  else registry.set(FIND_MATCH_HIGHLIGHT, new Highlight(...matches));
  if (actives.length === 0) registry.delete(FIND_ACTIVE_HIGHLIGHT);
  else registry.set(FIND_ACTIVE_HIGHLIGHT, new Highlight(...actives));
}

export function useSessionFindHighlights(
  contentRef: RefObject<HTMLElement | null>,
  query: string,
  activeMessageId: string | null
): void {
  const hostId = useId();
  useEffect(() => {
    const registry = highlightRegistry();
    const root = contentRef.current;
    const needle = query.trim();
    if (!registry) return undefined;
    if (!root || needle.length === 0) {
      hostRanges.delete(hostId);
      paintUnion(registry);
      return undefined;
    }
    let frame = 0;
    const paint = () => {
      frame = 0;
      hostRanges.set(hostId, collectFindRanges(root, needle, activeMessageId));
      paintUnion(registry);
    };
    const schedule = () => {
      if (frame !== 0) return;
      frame = requestAnimationFrame(paint);
    };
    paint();
    const observer = new MutationObserver(schedule);
    observer.observe(root, { characterData: true, childList: true, subtree: true });
    return () => {
      observer.disconnect();
      if (frame !== 0) cancelAnimationFrame(frame);
      hostRanges.delete(hostId);
      paintUnion(registry);
    };
  }, [activeMessageId, contentRef, hostId, query]);
}
