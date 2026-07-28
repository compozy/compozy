import { createContext, use, type Dispatch, type SetStateAction } from "react";
import { flushSync } from "react-dom";

export type TimelineExpansionSetter = Dispatch<SetStateAction<Set<string>>>;

// Shared per-timeline row state fed through context so the row renderer stays
// referentially stable (no closure deps) and each memoized `TimelineRowContent`
// skips re-render when its structurally-shared row ref is unchanged. Expansion
// state + toggle callbacks propagate through the memo boundary instead of riding
// fresh render closures.
export interface TimelineRowSharedState {
  expandedTurns: ReadonlySet<string>;
  setExpandedWorkGroups: TimelineExpansionSetter;
  setExpandedTurns: TimelineExpansionSetter;
  setExpandedChangedFiles: TimelineExpansionSetter;
}

export const TimelineRowContext = createContext<TimelineRowSharedState | null>(null);

export function useTimelineRowContext(): TimelineRowSharedState {
  const context = use(TimelineRowContext);
  if (!context) {
    throw new Error("Timeline row views must render within a TimelineRowContext provider");
  }
  return context;
}

export function toggleTimelineExpansion(
  setter: TimelineExpansionSetter,
  id: string,
  button: HTMLElement | null
): void {
  const viewport = button?.closest<HTMLElement>('[data-testid="chat-view"]');
  const previousHeight = viewport?.scrollHeight ?? 0;
  const previousTop = viewport?.scrollTop ?? 0;
  flushSync(() => {
    setter(previous => {
      const next = new Set(previous);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  });
  if (!viewport) return;
  viewport.scrollTop = previousTop + viewport.scrollHeight - previousHeight;
}
