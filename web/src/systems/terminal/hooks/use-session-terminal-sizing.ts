import type { TerminalDimensions, TerminalViewHandle } from "@compozy/ui";
import { useEffect, useState, type RefObject } from "react";

import { SESSION_TERMINAL_MAX_ROWS, sessionTerminalRows } from "../lib/session-terminal-rows";

/** Columns assumed before the emulator can measure its container. */
const FALLBACK_COLUMNS = 80;

/**
 * Sizes the screen once the emulator can measure its width, again whenever the
 * recorded output changes, and again when a new proposal arrives — a Code font
 * change re-measures the cell, so the same width holds a different column
 * count. A streaming screen keeps the full window.
 */
export function useSessionTerminalSizing({
  instanceId,
  handleRef,
  fullWindow,
  preview,
}: {
  instanceId: string;
  handleRef: RefObject<TerminalViewHandle | null>;
  fullWindow: boolean;
  preview: string;
}) {
  const [attachedFor, setAttachedFor] = useState<string | null>(null);
  const [proposedColumns, setProposedColumns] = useState<number | null>(null);
  const attached = attachedFor === instanceId;
  useEffect(() => {
    if (!attached) return;
    const handle = handleRef.current;
    const columns = proposedColumns ?? handle?.proposeDimensions()?.cols ?? FALLBACK_COLUMNS;
    handle?.applyDimensions({
      cols: columns,
      rows: fullWindow ? SESSION_TERMINAL_MAX_ROWS : sessionTerminalRows(preview, columns),
    });
  }, [attached, fullWindow, preview, proposedColumns, handleRef]);
  return {
    onAttached: () => setAttachedFor(instanceId),
    onProposeDimensions: (dimensions: TerminalDimensions) => setProposedColumns(dimensions.cols),
  };
}
