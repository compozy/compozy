import { useState } from "react";

import { PillCount, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { dockItemAccessibleName, type OsDockItemData } from "../lib/os-dock-model";
import { DockIcon } from "./os-dock-icons";

export type { OsDockItemData } from "../lib/os-dock-model";

/** Tip clearance beside the rail item (shell-rail v2 `.tip`). */
const RAIL_TIP_SIDE_OFFSET = 10;

const DOCK_ITEM_SELECTOR = '[data-slot="os-dock-item"]:not(:disabled)';

/**
 * The rail: the dock as a vertical launcher column on the chrome surface,
 * beside the desktop (shell-rail v2 `.rail` / `.ri`). DesktopDock owns the
 * runtime wiring; the foot slot hosts shell controls below the launchers.
 */
export interface OsDockProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  items: OsDockItemData[];
  /** Item activation. Omit to render items as presentation. */
  onSelect?: (id: string) => void;
  /** Keeps launchers visible while the authoritative command fence hydrates. */
  disabled?: boolean;
  /** Wraps each interactive item with its destination context menu (US-006). */
  renderItemMenu?: (item: OsDockItemData, children: React.ReactNode) => React.ReactNode;
  /** Shell controls pinned to the bottom of the rail, below the launchers. */
  foot?: React.ReactNode;
  /** Names the launcher navigation (not the rail container). */
  "aria-label"?: string;
}

type DockItemState = "minimized" | "running" | "closed";

function dockItemState(item: OsDockItemData): DockItemState {
  if (item.minimized) return "minimized";
  return item.running ? "running" : "closed";
}

function DockItemBody({ item }: { item: OsDockItemData }) {
  const state = dockItemState(item);
  return (
    <>
      <DockIcon name={item.icon} className={cn("size-5", item.minimized && "opacity-50")} />
      <span
        data-slot="os-dock-indicator"
        aria-hidden="true"
        className={cn(
          "absolute top-1/2 -left-1.5 -translate-y-1/2 rounded-full transition-opacity duration-base",
          state === "closed" ? "opacity-0" : "opacity-100",
          state === "minimized"
            ? "size-1.25 border border-subtle bg-transparent"
            : cn("size-1", item.active ? "bg-fg" : "bg-muted")
        )}
      />
      {item.badge ? (
        <PillCount
          data-slot="os-dock-badge"
          count={item.badge}
          className="absolute top-0.75 right-0.75 ring-2 ring-rail"
        />
      ) : null}
    </>
  );
}

const ITEM_BASE =
  "relative grid size-rail-item shrink-0 place-items-center rounded-lg text-muted transition-[background-color,color,box-shadow] duration-base ease-spring";
const ITEM_INTERACTIVE =
  "hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none";
const ITEM_ACTIVE = "bg-selected text-fg shadow-card hover:bg-selected";

function DockItem({
  item,
  tabStop,
  onFocusItem,
  onSelect,
  disabled,
  renderItemMenu,
}: {
  item: OsDockItemData;
  tabStop: boolean;
  onFocusItem: (id: string) => void;
  onSelect?: (id: string) => void;
  disabled?: boolean;
  renderItemMenu?: OsDockProps["renderItemMenu"];
}) {
  const name = dockItemAccessibleName(item);
  const classes = cn(ITEM_BASE, onSelect && ITEM_INTERACTIVE, item.active && ITEM_ACTIVE);
  const tip = (
    <TooltipContent side="right" sideOffset={RAIL_TIP_SIDE_OFFSET}>
      {item.name}
    </TooltipContent>
  );

  if (!onSelect) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              data-slot="os-dock-item"
              data-app={item.id}
              data-state={dockItemState(item)}
              className={classes}
            />
          }
        >
          <DockItemBody item={item} />
        </TooltipTrigger>
        {tip}
      </Tooltip>
    );
  }

  const interactiveItem = (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            data-slot="os-dock-item"
            data-app={item.id}
            data-state={dockItemState(item)}
            aria-label={name}
            aria-current={item.active ? "true" : undefined}
            tabIndex={tabStop ? 0 : -1}
            disabled={disabled}
            className={classes}
            onFocus={() => onFocusItem(item.id)}
            onClick={() => onSelect(item.id)}
          />
        }
      >
        <DockItemBody item={item} />
      </TooltipTrigger>
      {tip}
    </Tooltip>
  );
  return renderItemMenu ? <>{renderItemMenu(item, interactiveItem)}</> : interactiveItem;
}

export interface OsRailButtonProps extends Omit<React.ComponentProps<"button">, "children"> {
  /** Accessible name and the tooltip beside the rail. */
  label: string;
  /** Tooltip side: beside the rail, or above the compact tab bar. */
  tipSide?: "right" | "top";
  children: React.ReactNode;
}

/** A rail-foot control in the launcher grammar: 40px item, muted glyph, tooltip at the right. */
export function OsRailButton({
  label,
  tipSide = "right",
  className,
  children,
  ...props
}: OsRailButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            aria-label={label}
            className={cn(ITEM_BASE, ITEM_INTERACTIVE, className)}
            {...props}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent side={tipSide} sideOffset={RAIL_TIP_SIDE_OFFSET}>
        {label}
      </TooltipContent>
    </Tooltip>
  );
}

/** Up/Down roving focus across the launchers, wrapping at the ends; Home/End jump. */
function moveRovingFocus(event: React.KeyboardEvent<HTMLElement>): void {
  const items = Array.from(event.currentTarget.querySelectorAll<HTMLElement>(DOCK_ITEM_SELECTOR));
  const index = items.findIndex(item => item === event.target);
  const targets: Record<string, number> = {
    ArrowDown: index + 1,
    ArrowUp: index - 1,
    Home: 0,
    End: items.length - 1,
  };
  const target = targets[event.key];
  if (index < 0 || target === undefined) return;
  event.preventDefault();
  items[(target + items.length) % items.length]?.focus();
}

export function OsDock({
  items,
  onSelect,
  disabled,
  renderItemMenu,
  foot,
  "aria-label": ariaLabel = "Dock",
  className,
  ...props
}: OsDockProps) {
  const [focusedId, setFocusedId] = useState<string | null>(null);
  // One tab stop for the whole rail: the last focused launcher, else the
  // focused app's launcher, else the first.
  const tabStopId =
    (focusedId !== null && items.some(item => item.id === focusedId) ? focusedId : null) ??
    items.find(item => item.active)?.id ??
    items[0]?.id;

  return (
    <div
      data-slot="os-rail"
      className={cn(
        "flex w-rail shrink-0 flex-col items-center border-r border-line bg-rail",
        className
      )}
      {...props}
    >
      <nav
        data-slot="os-dock"
        aria-label={ariaLabel}
        className="no-scrollbar flex min-h-0 w-full flex-1 flex-col items-center gap-1.5 overflow-y-auto py-3.5"
        onKeyDown={onSelect ? moveRovingFocus : undefined}
      >
        {items.map(item => (
          <DockItem
            key={item.id}
            item={item}
            tabStop={item.id === tabStopId}
            onFocusItem={setFocusedId}
            onSelect={onSelect}
            disabled={disabled}
            renderItemMenu={renderItemMenu}
          />
        ))}
      </nav>
      {foot ? (
        <div
          data-slot="os-rail-foot"
          className="flex shrink-0 flex-col items-center gap-1.5 pt-2.5 pb-3.5"
        >
          {foot}
        </div>
      ) : null}
    </div>
  );
}
