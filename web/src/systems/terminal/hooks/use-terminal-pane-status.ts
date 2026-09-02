import { useEffect, useState } from "react";

import { exitNoticeFromTerminal } from "../lib/terminal-exit";
import type { TerminalPaneState } from "../stores/terminal-store";
import type { TerminalInfo } from "../types";

const RETENTION_REFRESH_MS = 30_000;

export function useTerminalPaneStatus(terminal: TerminalInfo, pane: TerminalPaneState | undefined) {
  // A terminal that exited before this pane attached has no live EXIT frame to
  // observe. Its outcome is still the daemon's, recorded on the terminal.
  const exit = pane?.exit ?? exitNoticeFromTerminal(terminal);
  const [retentionNow, setRetentionNow] = useState(() => Date.now());
  useEffect(() => {
    if (exit === null) return undefined;
    const timer = window.setInterval(() => setRetentionNow(Date.now()), RETENTION_REFRESH_MS);
    return () => window.clearInterval(timer);
  }, [exit]);

  const status = pane?.status ?? "connecting";
  const awaitingFirstFrame = status === "connecting" || status === "idle";
  const showConnecting =
    exit === null &&
    (status === "connecting" || status === "reconnecting" || status === "resyncing");
  const viewers = pane?.viewers ?? terminal.viewers;
  const settledCols = pane?.status === "connected" ? pane.cols : null;
  const settledRows = pane?.status === "connected" ? pane.rows : null;

  return {
    awaitingFirstFrame,
    exit,
    retentionNow,
    settledCols,
    settledRows,
    showConnecting,
    showSizeVote: settledCols !== null && settledRows !== null && viewers > 1,
    status,
  };
}
