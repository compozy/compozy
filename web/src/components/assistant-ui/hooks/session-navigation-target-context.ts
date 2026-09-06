import { createContext, use } from "react";

/**
 * What the navigation host asks the rendered rows to show (S8): the committed
 * find query for the in-transcript marks, which row holds the active match,
 * how many matches sit behind each settled fold, and — after a jump — the
 * exact part the reader is being taken to, so every disclosure on the way
 * (fold, work group, tool body, reasoning body) opens for it. Absent outside a
 * navigating viewport (stories, read-only views).
 */
export interface SessionNavigationReveal {
  /** Increments per jump; the reader's releases belong to one reveal. */
  key: number;
  messageId: string;
  /** The daemon's `turn_id` for the match (the entry's turn for a trail jump). */
  turnId: string;
  /** The daemon's `part_index`; `null` when the match carries no source (older daemon). */
  partIndex: number | null;
  field: string | null;
  toolCallId: string | null;
  /** The matched field sits in a collapsed body (reasoning, tool input/output/error). */
  opensBody: boolean;
}

export interface SessionNavigationTarget {
  /** The committed find query; empty when find is closed. */
  query: string;
  activeMessageId: string | null;
  /** Matches the daemon found behind a turn's fold, by `turn_id`. */
  foldMatchCounts: ReadonlyMap<string, number>;
  reveal: SessionNavigationReveal | null;
  /** Disclosure ids the reader closed by hand during this reveal; their hold is gone. */
  released: ReadonlySet<string>;
  releaseDisclosure: (id: string) => void;
}

export const SessionNavigationTargetContext = createContext<SessionNavigationTarget | null>(null);

export function useOptionalSessionNavigationTarget(): SessionNavigationTarget | null {
  return use(SessionNavigationTargetContext);
}

/**
 * A disclosure's hold from the current reveal: held while the reveal targets
 * something inside it and the reader has not closed it by hand. The reader's
 * own open/closed choice is layered underneath and never touched.
 */
export function useRevealHold(
  id: string,
  contains: (reveal: SessionNavigationReveal) => boolean
): { held: boolean; release: () => void } {
  const target = use(SessionNavigationTargetContext);
  const reveal = target?.reveal ?? null;
  const held = reveal !== null && !target!.released.has(id) && contains(reveal);
  return {
    held,
    release: () => target?.releaseDisclosure(id),
  };
}
