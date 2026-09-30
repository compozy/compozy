import { useState } from "react";

import { PillCount } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { dockTabStopId, moveDockRovingFocus } from "../lib/dock-roving-focus";
import { dockItemAccessibleName, type OsDockItemData } from "../lib/os-dock-model";
import { DockIcon } from "./os-dock-icons";

/** Counts cap at "9+" without collapsing the zero/non-zero distinction. */
function TabBarItem({
  item,
  tabStop,
  onFocusItem,
  onSelect,
  disabled,
}: {
  item: OsDockItemData;
  tabStop: boolean;
  onFocusItem: (id: string) => void;
  onSelect: (id: string) => void;
  disabled?: boolean;
}) {
  const state = item.minimized ? "minimized" : item.running ? "running" : "closed";
  return (
    <button
      type="button"
      data-slot="os-dock-item"
      data-app={item.id}
      data-state={state}
      aria-label={dockItemAccessibleName(item)}
      aria-current={item.active ? "true" : undefined}
      tabIndex={tabStop ? 0 : -1}
      disabled={disabled}
      className={cn(
        "relative grid size-rail-item shrink-0 place-items-center rounded-lg text-muted",
        "transition-[background-color,color,box-shadow] duration-base ease-spring",
        "hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
        item.active && "bg-selected text-fg shadow-card hover:bg-selected"
      )}
      onFocus={() => onFocusItem(item.id)}
      onClick={() => onSelect(item.id)}
    >
      <DockIcon name={item.icon} className={cn("size-5", item.minimized && "opacity-50")} />
      {item.badge ? (
        <PillCount
          data-slot="os-dock-badge"
          count={item.badge}
          className="absolute top-0.75 right-0.75 ring-2 ring-rail"
        />
      ) : null}
      <span
        data-slot="os-dock-indicator"
        aria-hidden="true"
        className={cn(
          "absolute -bottom-1.5 left-1/2 -translate-x-1/2 rounded-full transition-opacity duration-base",
          state === "closed" ? "opacity-0" : "opacity-100",
          state === "minimized"
            ? "size-1.25 border border-subtle bg-transparent"
            : cn("size-1", item.active ? "bg-fg" : "bg-muted")
        )}
      />
    </button>
  );
}

export interface OsDockTabBarProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  items: OsDockItemData[];
  onSelect: (id: string) => void;
  disabled?: boolean;
  /** Shell controls after the launchers (the rail foot, which compact hides). */
  trailing?: React.ReactNode;
}

/**
 * Compact (<960px) dock: a full-width bottom tab bar in its own shell-grid row
 * below the desktop, in the rail's item grammar laid out horizontally — a
 * scrollable launcher strip with full parity, then the rail-foot controls
 * behind a hairline. Safe-area aware.
 */
export function OsDockTabBar({
  items,
  onSelect,
  disabled,
  trailing,
  className,
  ...props
}: OsDockTabBarProps) {
  const [focusedId, setFocusedId] = useState<string | null>(null);
  const tabStopId = dockTabStopId(items, focusedId);
  return (
    <div
      data-slot="os-dock-tabbar"
      className={cn(
        "flex items-stretch border-t border-line bg-rail pb-[env(safe-area-inset-bottom,0px)]",
        className
      )}
      {...props}
    >
      <nav
        aria-label="Dock"
        className="no-scrollbar flex min-w-0 flex-1 items-center gap-1.5 overflow-x-auto overscroll-x-contain px-2 py-2"
        style={{ paddingInlineStart: "calc(var(--spacing) * 2 + env(safe-area-inset-left, 0px))" }}
        onKeyDown={event => moveDockRovingFocus(event, "horizontal")}
      >
        {items.map(item => (
          <TabBarItem
            key={item.id}
            item={item}
            tabStop={item.id === tabStopId}
            onFocusItem={setFocusedId}
            onSelect={onSelect}
            disabled={disabled}
          />
        ))}
      </nav>
      {trailing ? (
        <div
          data-slot="os-dock-tabbar-trailing"
          className="flex shrink-0 items-center gap-1.5 border-l border-line py-2 pl-2"
          style={{
            paddingInlineEnd: "calc(var(--spacing) * 2 + env(safe-area-inset-right, 0px))",
          }}
        >
          {trailing}
        </div>
      ) : null}
    </div>
  );
}
