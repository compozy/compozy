import { ChevronRight } from "lucide-react";
import type { ReactNode } from "react";

import { Icon, cn } from "@compozy/ui";

export interface SessionListGroupToggleProps {
  collapsed: boolean;
  count: number;
  identity: ReactNode;
  onToggle?: () => void;
  /** What a collapsed group still has to say about its rows. */
  signal?: ReactNode;
  testId?: string;
}

/** A group header's collapse toggle: chevron, identity, collapsed signal and count. */
export function SessionListGroupToggle({
  collapsed,
  count,
  identity,
  onToggle,
  signal,
  testId,
}: SessionListGroupToggleProps) {
  return (
    <button
      type="button"
      aria-expanded={!collapsed}
      data-testid={testId}
      className="flex min-h-7 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 py-1 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
      onClick={onToggle}
    >
      <Icon
        as={ChevronRight}
        size="sm"
        className={cn("shrink-0 text-faint transition-transform", !collapsed && "rotate-90")}
      />
      <span className="flex min-w-0 flex-1 items-center">{identity}</span>
      {collapsed ? signal : null}
      <span className="font-mono text-micro text-faint">{count}</span>
    </button>
  );
}
