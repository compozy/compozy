import type { ReactNode } from "react";

import { cn } from "@compozy/ui";

import { onDisclosureArrowKey } from "../../lib/session-list-stops";
import {
  SESSION_LIST_DISCLOSURE_GROUP,
  SessionListDisclosureMark,
} from "./session-list-disclosure-mark";

export interface SessionListGroupToggleProps {
  collapsed: boolean;
  /** The rows the group holds; `null` shows none (a search narrows what it would count). */
  count: number | string | null;
  identity: ReactNode;
  onToggle?: () => void;
  /** What a collapsed group still has to say about its rows, after its identity. */
  signal?: ReactNode;
  /** A collapsed group's running signal, in the leading column (#1926). */
  running?: ReactNode;
  testId?: string;
}

/** A group header's collapse toggle: chevron (or running signal), identity, collapsed signal and count. */
export function SessionListGroupToggle({
  collapsed,
  count,
  identity,
  onToggle,
  signal,
  running,
  testId,
}: SessionListGroupToggleProps) {
  return (
    <button
      type="button"
      aria-expanded={!collapsed}
      data-testid={testId}
      data-sidebar-stop="group"
      className={cn(
        SESSION_LIST_DISCLOSURE_GROUP,
        "flex min-h-7 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
      onClick={onToggle}
      onKeyDown={event => onDisclosureArrowKey(event, !collapsed, onToggle)}
    >
      <SessionListDisclosureMark open={!collapsed} signal={running} />
      <span className="flex min-w-0 flex-1 items-center">{identity}</span>
      {collapsed ? signal : null}
      {count === null ? null : <span className="font-mono text-micro text-faint">{count}</span>}
    </button>
  );
}
