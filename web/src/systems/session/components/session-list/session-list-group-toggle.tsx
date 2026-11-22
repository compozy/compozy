import type { ReactNode } from "react";

import { onDisclosureArrowKey } from "../../lib/session-list-stops";
import {
  SessionListDisclosureMark,
  type SessionListLeadingSignal,
} from "./session-list-disclosure-mark";

export interface SessionListGroupToggleProps {
  collapsed: boolean;
  /** The rows the group holds; `null` shows none (a search narrows what it would count). */
  count: number | string | null;
  identity: ReactNode;
  onToggle?: () => void;
  /** What a collapsed group still has to say about its rows, after its identity. */
  signal?: ReactNode;
  /** A collapsed group's needs-you and running counts, for its leading glyph (#1926). */
  leading?: SessionListLeadingSignal;
  testId?: string;
}

/** A group header's collapse toggle: chevron (or leading state glyph), identity, collapsed signal and count. */
export function SessionListGroupToggle({
  collapsed,
  count,
  identity,
  onToggle,
  signal,
  leading,
  testId,
}: SessionListGroupToggleProps) {
  return (
    <button
      type="button"
      aria-expanded={!collapsed}
      data-testid={testId}
      data-sidebar-stop="group"
      className="group/disclosure flex min-h-7 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
      onClick={onToggle}
      onKeyDown={event => onDisclosureArrowKey(event, !collapsed, onToggle)}
    >
      <SessionListDisclosureMark open={!collapsed} signal={leading} />
      <span className="flex min-w-0 flex-1 items-center">{identity}</span>
      {collapsed ? signal : null}
      {count === null ? null : <span className="font-mono text-micro text-faint">{count}</span>}
    </button>
  );
}
