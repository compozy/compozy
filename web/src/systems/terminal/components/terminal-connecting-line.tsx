import { Spinner } from "@compozy/ui";

import type { TerminalStreamStatus } from "../lib/terminal-protocol-client";

/**
 * What the connection is doing, said once and quietly.
 *
 * Three states share this line because they are the same fact to a reader: the
 * screen is not live yet. It never fills the grid with placeholder content —
 * waiting is not allowed to look like output — and it says nothing at all once
 * the stream is connected.
 */
const STATUS_COPY: Partial<Record<TerminalStreamStatus, string>> = {
  connecting: "Connecting…",
  reconnecting: "Reconnecting…",
  // The screen is being rebuilt from the server after skipped output.
  resyncing: "Catching up…",
};

export function TerminalConnectingLine({ status }: { status: TerminalStreamStatus }) {
  const label = STATUS_COPY[status];
  if (!label) return null;
  return (
    <div
      className="flex items-center gap-2 px-3.5 py-2.5 text-form-input text-subtle"
      data-status={status}
      data-testid="terminal-connecting"
      role="status"
    >
      <Spinner aria-hidden="true" className="size-3" />
      {label}
    </div>
  );
}
