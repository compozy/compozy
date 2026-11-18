/** What follows the pointer during a drag: the tab plate, or a window's head chip. */
import { cn } from "@compozy/ui";

import type { DraftTab, DraftWindow } from "./model";
import { leafIds } from "./model";
import { APP_NAME } from "./apps";
import { OsTabTiles } from "./os-tab";

export type DragGhostSource = { kind: "tab"; tab: DraftTab } | { kind: "head"; windowId: string };

export interface DragGhostProps {
  source: DragGhostSource;
  windows: Record<string, DraftWindow>;
  /** Pointer position in px from the shell's top-left corner. */
  x: number;
  y: number;
  /** The drop would do nothing here (US-007.EC-2, US-007.EC-4). */
  refused?: boolean;
}

export function DragGhost({ source, windows, x, y, refused = false }: DragGhostProps) {
  const members =
    source.kind === "tab"
      ? leafIds(source.tab.root).map(id => windows[id]!)
      : [windows[source.windowId]!];
  const lead = members[0]!;
  return (
    <div
      className={cn(
        "pointer-events-none absolute z-50 flex h-6.75 max-w-56 items-center gap-2 rounded-sm bg-elevated pr-3 pl-[3.5px] text-body font-medium text-fg shadow-pop",
        refused && "opacity-60"
      )}
      data-slot="os-drag-ghost"
      style={{ left: x + 10, top: y + 8 }}
    >
      <OsTabTiles windows={members} />
      <span className="truncate">{source.kind === "tab" ? lead.title : APP_NAME[lead.app]}</span>
      {members.length > 1 ? (
        <span className="text-meta text-subtle tabular-nums">{members.length}</span>
      ) : null}
    </div>
  );
}
