import { StateGlyph, cn } from "@compozy/ui";
import type { PointerEvent } from "react";

import type { SessionPayload } from "@/systems/session";

import type { OsWindow } from "../lib/os-types";
import { useOsWindowTitle } from "../hooks/use-os-window-title";
import { sessionSignal, tabSignal } from "../lib/tab-signal";
import { OsTabTiles } from "./os-tab";

export interface DesktopOverviewTabRowProps {
  tabId: string;
  /** Members in reading order, lead first. */
  members: readonly OsWindow[];
  /** The tab this client shows on its desktop. */
  active: boolean;
  disabled?: boolean;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  onChoose: () => void;
  /** Starts a drag of the whole tab toward another desktop's card. */
  onPointerDown?: (event: PointerEvent<HTMLElement>) => void;
}

/**
 * One tab on a desktop card: its icon tiles, the lead window's title, the member count of a
 * split and the loudest signal. Choosing it jumps to the tab; dragging it moves the tab.
 */
export function DesktopOverviewTabRow({
  tabId,
  members,
  active,
  disabled = false,
  sessionOf,
  onChoose,
  onPointerDown,
}: DesktopOverviewTabRowProps) {
  const lead = members[0]!;
  const title = useOsWindowTitle(lead, sessionOf(lead));
  const signal = tabSignal(members.map(member => sessionSignal(sessionOf(member))));
  const label = members.length > 1 ? `${title} and ${members.length - 1} more` : title;
  return (
    <button
      aria-current={active ? "true" : undefined}
      aria-label={`Go to ${label}`}
      className={cn(
        "flex h-7 w-full min-w-0 items-center gap-2 rounded-sm px-1 text-left text-small-body outline-none select-none hover:bg-fg/7 focus-visible:shadow-focus-ring disabled:cursor-not-allowed",
        active ? "text-fg" : "text-fg-2"
      )}
      data-slot="desktop-overview-tab"
      data-tab-id={tabId}
      disabled={disabled}
      onClick={onChoose}
      onPointerDown={onPointerDown}
      type="button"
    >
      <OsTabTiles members={members} sessionOf={sessionOf} />
      <span className="min-w-0 flex-1 truncate">{title}</span>
      {members.length > 1 ? (
        <span className="text-meta text-subtle tabular-nums">{members.length}</span>
      ) : null}
      {signal ? <StateGlyph label={signal.label} size="sm" state={signal.state} /> : null}
    </button>
  );
}
