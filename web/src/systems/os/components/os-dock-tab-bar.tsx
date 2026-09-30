import { Plus } from "lucide-react";

import { Icon, PillCount } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { dockItemAccessibleName, type OsDockItemData } from "../lib/os-dock-model";
import { DockIcon } from "./os-dock-icons";

/** Counts cap at "9+" without collapsing the zero/non-zero distinction. */
function TabBarItem({
  item,
  onSelect,
  disabled,
}: {
  item: OsDockItemData;
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
      disabled={disabled}
      className={cn(
        "relative grid size-dock-tab-item shrink-0 place-items-center rounded-lg",
        "transition-colors duration-shell-fast",
        "hover:bg-btn-default-fill focus-visible:shadow-focus-ring focus-visible:outline-none"
      )}
      onClick={() => onSelect(item.id)}
    >
      <span
        className={cn(
          "grid place-items-center text-muted transition-colors duration-base",
          item.minimized && "opacity-55"
        )}
      >
        <DockIcon name={item.icon} className="size-dock-icon" />
      </span>
      {item.badge ? (
        <PillCount
          data-slot="os-dock-badge"
          count={item.badge}
          className="absolute top-0.5 right-0.5"
        />
      ) : null}
      <span
        data-slot="os-dock-indicator"
        aria-hidden="true"
        className={cn(
          "absolute left-1/2 -translate-x-1/2 rounded-full transition-opacity duration-base",
          item.running || item.minimized ? "opacity-100" : "opacity-0",
          item.minimized
            ? "bottom-0.5 size-dock-indicator-min border border-muted bg-transparent"
            : "bottom-[3px] size-dock-indicator bg-muted"
        )}
      />
    </button>
  );
}

export interface OsDockTabBarProps extends Omit<React.ComponentProps<"nav">, "onSelect"> {
  items: OsDockItemData[];
  onSelect: (id: string) => void;
  disabled?: boolean;
  onNewSession: () => void;
}

/**
 * Compact (<960px) dock: a full-width bottom tab bar in its own shell-grid
 * row below the desktop — horizontally scrollable strip with full launcher
 * parity, no tooltips, safe-area aware. New Session keeps its own segment
 * behind a leading hairline.
 */
export function OsDockTabBar({
  items,
  onSelect,
  disabled,
  onNewSession,
  className,
  ...props
}: OsDockTabBarProps) {
  return (
    <nav
      data-slot="os-dock-tabbar"
      aria-label="Dock"
      className={cn("flex items-stretch border-t border-line bg-rail", className)}
      {...props}
    >
      <div
        className={cn(
          "no-scrollbar flex min-w-0 flex-1 items-center gap-0.5 overflow-x-auto",
          "overscroll-x-contain px-1.5 pt-1.5 pb-[calc(--spacing(1.5)+env(safe-area-inset-bottom,0px))]"
        )}
      >
        {items.map(item => (
          <TabBarItem key={item.id} item={item} onSelect={onSelect} disabled={disabled} />
        ))}
      </div>
      <div
        data-slot="os-dock-actions"
        className="flex shrink-0 items-center border-l border-line py-1.5 pr-2 pb-[calc(--spacing(1.5)+env(safe-area-inset-bottom,0px))] pl-1.5"
        style={{
          paddingInlineEnd: "calc(var(--spacing) * 2 + env(safe-area-inset-right, 0px))",
        }}
      >
        <button
          type="button"
          data-slot="os-dock-new"
          aria-label="New session"
          className={cn(
            "grid size-dock-tab-item place-items-center rounded-lg bg-accent text-accent-ink shadow-highlight",
            "transition-colors duration-shell-fast hover:bg-accent-hover",
            "focus-visible:shadow-focus-ring focus-visible:outline-none"
          )}
          onClick={onNewSession}
        >
          <Icon as={Plus} className="size-dock-new-icon" />
        </button>
      </div>
    </nav>
  );
}
