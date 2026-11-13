/**
 * `os-tab-bar` (S1): the one bar of the active desktop, on the chrome above the content. Pinned
 * tabs lead, regular tabs shrink to fit, then the new-tab button; when tabs hit the 40px floor
 * the strip scrolls and the overflow list names every tab.
 */
import { ChevronDown, Plus } from "@compozy/ui/icons";
import { Fragment, type ReactNode } from "react";

import { Button, cn } from "@compozy/ui";

import type { DraftTab, DraftWindow } from "./model";
import { OsTab, type OsTabProps } from "./os-tab";

/** The 2px marker where a dragged tab or window head would land. */
export function OsTabInsertion() {
  return (
    <span
      aria-hidden="true"
      className="h-6.75 w-0.5 shrink-0 rounded-pill bg-brand"
      data-slot="os-tab-insertion"
    />
  );
}

export interface OsTabBarProps {
  tabs: readonly DraftTab[];
  windows: Record<string, DraftWindow>;
  activeTabId: string;
  /** Per-tab forced presentation for static states. */
  tabProps?: Record<string, Partial<OsTabProps>>;
  /** Index (among all tabs) before which the insertion marker shows. */
  insertionIndex?: number | null;
  /** Tabs sit at the 40px floor and no longer fit: the list of every tab appears. */
  overflowing?: boolean;
  /** Extra controls at the end of the bar. */
  trailing?: ReactNode;
  compact?: boolean;
  onActivate?: (tabId: string) => void;
  onClose?: (tabId: string) => void;
  onNewTab?: () => void;
  onTabPointerDown?: (tabId: string, event: React.PointerEvent<HTMLElement>) => void;
  className?: string;
}

export function OsTabBar({
  tabs,
  windows,
  activeTabId,
  tabProps,
  insertionIndex = null,
  overflowing = false,
  trailing,
  compact = false,
  onActivate,
  onClose,
  onNewTab,
  onTabPointerDown,
  className,
}: OsTabBarProps) {
  const pinnedCount = tabs.filter(tab => tab.pinned).length;
  return (
    <div
      className={cn("flex h-10 min-w-0 items-center gap-1 bg-rail pr-2 pl-1.5", className)}
      data-slot="os-tab-bar"
    >
      <div
        className={cn("no-scrollbar flex min-w-0 flex-1 items-center gap-1 overflow-x-auto")}
        data-slot="os-tab-strip"
      >
        <div aria-label="Tabs" className="contents" role="tablist">
          {tabs.map((tab, index) => (
            <Fragment key={tab.id}>
              {insertionIndex === index ? <OsTabInsertion /> : null}
              {index === pinnedCount && pinnedCount > 0 ? (
                <span aria-hidden="true" className="mx-0.5 h-4 w-px shrink-0 bg-line-strong" />
              ) : null}
              <OsTab
                active={tab.id === activeTabId}
                onActivate={() => onActivate?.(tab.id)}
                onClose={() => onClose?.(tab.id)}
                onPointerDown={event => onTabPointerDown?.(tab.id, event)}
                tab={tab}
                windows={windows}
                {...tabProps?.[tab.id]}
              />
            </Fragment>
          ))}
          {insertionIndex === tabs.length ? <OsTabInsertion /> : null}
        </div>
        <Button
          aria-label="New tab"
          className="size-6.75 shrink-0 rounded-sm text-fg-2"
          onClick={onNewTab}
          size="icon-sm"
          type="button"
          variant="quiet"
        >
          <Plus aria-hidden="true" className="size-3.75" />
        </Button>
      </div>
      {overflowing ? (
        <Button
          aria-label={`All ${tabs.length} tabs`}
          className="h-6.75 shrink-0 gap-1 rounded-sm px-2 text-fg-2"
          size="sm"
          type="button"
          variant="quiet"
        >
          <span className="text-meta tabular-nums">{tabs.length}</span>
          <ChevronDown aria-hidden="true" className="size-3.5" />
        </Button>
      ) : null}
      {compact ? null : trailing}
    </div>
  );
}
