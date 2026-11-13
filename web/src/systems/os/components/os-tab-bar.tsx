import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@compozy/ui";
import { ChevronDown, Plus } from "@compozy/ui/icons";
import { Fragment, type PointerEvent } from "react";

import { cn } from "@/lib/utils";
import type { SessionPayload } from "@/systems/session";

import { useOsTabBar, type OsTabBarModel, type OsTabModel } from "../hooks/use-os-tab-bar";
import { useOsTabDrag, useOsTabDragStart } from "../hooks/use-os-tab-drag";
import type { OsWindow } from "../lib/os-types";
import type { TabDragState } from "../stores/os-tab-drag-store";
import { useOsWindowTitle } from "../hooks/use-os-window-title";
import { OsDragGhost } from "./os-drag-ghost";
import { OsTab } from "./os-tab";
import { OsTabMenu } from "./os-tab-menu";

function sessionLookup(sessionById: ReadonlyMap<string, SessionPayload>) {
  return (win: OsWindow) =>
    win.app === "session" ? sessionById.get(win.instanceKey ?? "") : undefined;
}

function OverflowEntry({
  model,
  sessionOf,
  onSelect,
}: {
  model: OsTabModel;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  onSelect: () => void;
}) {
  const lead = model.members[0]!;
  const title = useOsWindowTitle(lead, sessionOf(lead));
  return (
    <DropdownMenuItem onClick={onSelect} data-active={model.active ? "" : undefined}>
      <span className="min-w-0 flex-1 truncate">{title}</span>
      {model.members.length > 1 ? (
        <span className="shrink-0 text-meta text-subtle tabular-nums">{model.members.length}</span>
      ) : null}
    </DropdownMenuItem>
  );
}

/** Tabs sit at the floor and still do not fit: the list names every tab. */
function OsTabOverflow({
  bar,
  sessionOf,
}: {
  bar: OsTabBarModel;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
}) {
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`All ${bar.tabs.length} tabs`}
        data-slot="os-tab-overflow"
        render={
          <Button
            type="button"
            variant="quiet"
            size="sm"
            className="h-6.75 shrink-0 gap-1 rounded-sm px-2 text-fg-2"
          />
        }
      >
        <span className="text-meta tabular-nums">{bar.tabs.length}</span>
        <ChevronDown aria-hidden="true" className="size-3.5" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-96 min-w-56 overflow-y-auto">
        {bar.tabs.map(model => (
          <OverflowEntry
            key={model.tab.id}
            model={model}
            sessionOf={sessionOf}
            onSelect={() => bar.activate(model.tab.id)}
          />
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** The 2px marker where a dragged tab would land. */
function OsTabInsertion() {
  return (
    <span
      aria-hidden="true"
      className="h-6.75 w-0.5 shrink-0 rounded-pill bg-brand"
      data-slot="os-tab-insertion"
    />
  );
}

/** What a drag in progress shows on the bar; the idle bar shows none of it. */
export interface OsTabBarDragView {
  insertionIndex: number | null;
  draggingTabId: string | null;
  dwellingTabId: string | null;
}

const IDLE_DRAG: OsTabBarDragView = {
  insertionIndex: null,
  draggingTabId: null,
  dwellingTabId: null,
};

function barDragView(drag: TabDragState): OsTabBarDragView {
  if (drag.phase !== "dragging" || drag.session === null) return IDLE_DRAG;
  const { source } = drag.session;
  return {
    insertionIndex: drag.insertionIndex,
    draggingTabId: source.kind === "tab" ? source.tabId : null,
    dwellingTabId: drag.dwell?.tabId ?? null,
  };
}

/**
 * The active desktop's one bar, on the chrome above the content. Pinned tabs lead, regular tabs
 * shrink to fit, then the new-tab button; when tabs hit the floor the strip scrolls and the
 * overflow list names every tab.
 */
export function OsTabBarView({
  bar,
  drag = IDLE_DRAG,
  onTabPointerDown,
  className,
}: {
  bar: OsTabBarModel;
  drag?: OsTabBarDragView;
  onTabPointerDown?: (tabId: string, event: PointerEvent<HTMLElement>) => void;
  className?: string;
}) {
  const sessionOf = sessionLookup(bar.sessionById);
  const pinnedCount = bar.tabs.filter(model => model.tab.pinned).length;
  return (
    <div
      className={cn("flex h-10 min-w-0 items-center gap-1 bg-rail pr-2 pl-1.5", className)}
      data-slot="os-tab-bar"
    >
      <div
        ref={bar.stripRef}
        className="no-scrollbar flex min-w-0 flex-1 items-center gap-1 overflow-x-auto"
        data-slot="os-tab-strip"
      >
        <div aria-label="Tabs" className="contents" data-testid="os-tab-bar" role="tablist">
          {bar.tabs.map((model, index) => (
            <Fragment key={model.tab.id}>
              {drag.insertionIndex === index ? <OsTabInsertion /> : null}
              {index === pinnedCount && pinnedCount > 0 ? (
                <span aria-hidden="true" className="mx-0.5 h-4 w-px shrink-0 bg-line-strong" />
              ) : null}
              <OsTabMenu bar={bar} model={model} sessionOf={sessionOf}>
                <OsTab
                  tabId={model.tab.id}
                  members={model.members}
                  pinned={model.tab.pinned}
                  active={model.active}
                  sessionOf={sessionOf}
                  width={bar.widthByTabId.get(model.tab.id)}
                  dragging={drag.draggingTabId === model.tab.id}
                  dwelling={drag.dwellingTabId === model.tab.id}
                  onActivate={() => bar.activate(model.tab.id)}
                  onClose={() => bar.close(model.tab.id)}
                  onPointerDown={event => onTabPointerDown?.(model.tab.id, event)}
                />
              </OsTabMenu>
            </Fragment>
          ))}
          {drag.insertionIndex === bar.tabs.length ? <OsTabInsertion /> : null}
        </div>
        <Button
          type="button"
          variant="quiet"
          size="icon-sm"
          aria-label={`New tab${bar.shortcutLabels.newTab ? ` (${bar.shortcutLabels.newTab})` : ""}`}
          data-slot="os-tab-new"
          className="size-6.75 shrink-0 rounded-sm text-fg-2"
          onClick={bar.openNewTab}
        >
          <Plus aria-hidden="true" className="size-3.75" />
        </Button>
      </div>
      {bar.overflow ? <OsTabOverflow bar={bar} sessionOf={sessionOf} /> : null}
    </div>
  );
}

export type OsTabBarProps = { className?: string };

/** The live tab bar bound to the shell's window manager, with its drags and their ghost. */
export function OsTabBar({ className }: OsTabBarProps) {
  const bar = useOsTabBar();
  const drag = useOsTabDrag();
  const { startTabDrag } = useOsTabDragStart();
  return (
    <>
      <OsTabBarView
        bar={bar}
        drag={barDragView(drag)}
        onTabPointerDown={startTabDrag}
        className={className}
      />
      <OsDragGhost sessionOf={sessionLookup(bar.sessionById)} />
    </>
  );
}
