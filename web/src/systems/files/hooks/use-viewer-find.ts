import { useMemo, useState } from "react";

import type { CodeViewerMatch } from "@compozy/ui";

import { findInText, firstMatchFrom } from "../lib/find-in-text";
import type { ViewerFindSeed } from "../lib/file-viewer-target";

export interface UseViewerFindResult {
  open: boolean;
  query: string;
  matchCase: boolean;
  matches: readonly CodeViewerMatch[];
  /** Index of the current match, or `null` with no matches. */
  active: number | null;
  /** Bumps on every explicit open, so the bar refocuses its input. */
  focusToken: number;
  show: () => void;
  close: () => void;
  setQuery: (next: string) => void;
  setMatchCase: (next: boolean) => void;
  next: () => void;
  previous: () => void;
}

const NO_MATCHES: readonly CodeViewerMatch[] = [];

/**
 * Find in file over the viewer's full loaded text (never DOM search, since lines are
 * virtualized). Typing jumps to the first match; a seeded find starts at the opened line.
 */
export function useViewerFind(
  text: string,
  seed?: ViewerFindSeed,
  seedLine?: number
): UseViewerFindResult {
  const [open, setOpen] = useState(seed !== undefined);
  const [query, setQueryState] = useState(seed?.query ?? "");
  const [matchCase, setMatchCaseState] = useState(seed?.matchCase ?? false);
  const [focusToken, setFocusToken] = useState(0);
  // `null` until the operator steps; until then the first match (from the seed line) is current.
  const [stepped, setStepped] = useState<number | null>(null);
  const [anchorLine, setAnchorLine] = useState<number | undefined>(seed ? seedLine : undefined);

  const matches = useMemo(
    () => (open ? findInText(text, query, matchCase) : NO_MATCHES),
    [open, text, query, matchCase]
  );
  const initial = firstMatchFrom(matches, anchorLine);
  const active =
    matches.length === 0 ? null : stepped !== null && stepped < matches.length ? stepped : initial;

  const restart = () => {
    setStepped(null);
    setAnchorLine(undefined);
  };
  const step = (delta: number) => {
    if (matches.length === 0) return;
    const current = active ?? 0;
    setStepped((current + delta + matches.length) % matches.length);
  };

  return {
    open,
    query,
    matchCase,
    matches,
    active,
    focusToken,
    show: () => {
      setOpen(true);
      setFocusToken(token => token + 1);
    },
    close: () => {
      setOpen(false);
      setStepped(null);
    },
    setQuery: nextQuery => {
      setQueryState(nextQuery);
      restart();
    },
    setMatchCase: nextMatchCase => {
      setMatchCaseState(nextMatchCase);
      restart();
    },
    next: () => step(1),
    previous: () => step(-1),
  };
}
