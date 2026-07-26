import { useRef } from "react";
import { Plus } from "lucide-react";

import { Icon, Tooltip, TooltipContent, TooltipTrigger } from "@agh/ui";

import { cn } from "@/lib/utils";

import { useDockMagnify } from "../hooks/use-dock-magnify";
import { isOsDockSeparator, type OsDockEntry, type OsDockItemData } from "./os-dock-types";

/** OpenDesign tip clearance above the icon (`bottom: calc(100% + 12px)`). */
const DOCK_TIP_SIDE_OFFSET = 12;

export type { OsDockEntry, OsDockItemData, OsDockSeparator } from "./os-dock-types";

/**
 * The dock: a centered glass strip of app launchers floating over the desktop,
 * with an optional detached New Session control in its own glass segment
 * (OpenDesign `dock-zone` anatomy). DesktopDock owns the runtime wiring.
 */

export interface OsDockProps extends Omit<React.ComponentProps<"nav">, "onSelect"> {
  items: OsDockEntry[];
  /** Item activation. Omit to render items as presentation. */
  onSelect?: (id: string) => void;
  /**
   * OpenDesign proximity magnification. Default on; DesktopDock turns this
   * off in compact presentation. Reduced-motion also disables the effect.
   */
  magnify?: boolean;
}

interface OsDockNewSessionProps extends Omit<
  React.ComponentProps<"button">,
  "onClick" | "children"
> {
  /** When omitted the control renders as non-interactive presentation. */
  onNewSession?: () => void;
}

/** Counts cap at "9+" without collapsing the zero/non-zero distinction. */
function formatBadge(count: number): string {
  return count > 9 ? "9+" : String(count);
}

function DockTip({ label }: { label: string }) {
  return (
    <TooltipContent
      side="top"
      sideOffset={DOCK_TIP_SIDE_OFFSET}
      className={cn(
        "border border-line-strong bg-shell-glass-pop px-2.5 py-1 text-micro font-medium shadow-none",
        // OpenDesign dock tip has no caret; hide the shared Tooltip arrow.
        "[&>*:last-child]:hidden"
      )}
    >
      {label}
    </TooltipContent>
  );
}

function DockItem({ item, onSelect }: { item: OsDockItemData; onSelect?: (id: string) => void }) {
  const state = item.minimized ? "minimized" : item.running ? "running" : "closed";
  const body = (
    <>
      <span
        className={cn(
          "grid place-items-center text-muted transition-colors duration-base",
          item.minimized && "opacity-55"
        )}
      >
        <item.icon className="size-dock-icon" />
      </span>
      {item.badge ? (
        <span
          data-slot="os-dock-badge"
          className="absolute top-0.5 right-0.5 grid h-dock-badge min-w-dock-badge place-items-center rounded-lg bg-accent px-1 font-mono text-micro font-bold text-accent-ink"
        >
          {formatBadge(item.badge)}
        </span>
      ) : null}
      <span
        data-slot="os-dock-indicator"
        aria-hidden="true"
        className={cn(
          "absolute bottom-0 left-1/2 -translate-x-1/2 rounded-full transition-opacity duration-base",
          item.running || item.minimized ? "opacity-100" : "opacity-0",
          item.minimized
            ? "size-dock-indicator-min border border-muted bg-transparent"
            : "size-dock-indicator bg-muted"
        )}
      />
    </>
  );

  const base =
    "relative grid size-dock-item origin-bottom place-items-center rounded-dock-item transition-[transform,background-color,color] duration-shell-fast ease-spring";
  const interactive =
    "hover:bg-btn-default-fill focus-visible:shadow-focus-ring focus-visible:outline-none";
  const classes = cn(base, onSelect && interactive);

  if (!onSelect) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <span
              data-slot="os-dock-item"
              data-app={item.id}
              data-state={state}
              className={classes}
            />
          }
        >
          {body}
        </TooltipTrigger>
        <DockTip label={item.name} />
      </Tooltip>
    );
  }
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <button
            type="button"
            data-slot="os-dock-item"
            data-app={item.id}
            data-state={state}
            aria-label={item.name}
            className={classes}
            onClick={() => onSelect(item.id)}
          />
        }
      >
        {body}
      </TooltipTrigger>
      <DockTip label={item.name} />
    </Tooltip>
  );
}

const DOCK_SEG =
  "flex items-end gap-dock-gap rounded-dock border border-line bg-shell-glass p-dock-pad shadow-dock backdrop-blur-shell";

export function OsDock({ items, onSelect, magnify = true, className, ...props }: OsDockProps) {
  const rootRef = useRef<HTMLElement>(null);
  useDockMagnify(rootRef, magnify);

  return (
    <nav
      ref={rootRef}
      data-slot="os-dock"
      aria-label="Dock"
      className={cn(DOCK_SEG, className)}
      {...props}
    >
      {items.map(entry =>
        isOsDockSeparator(entry) ? (
          <span
            key={entry.id}
            data-slot="os-dock-sep"
            aria-hidden="true"
            className="mb-dock-pad h-8 w-px shrink-0 self-end bg-line-strong"
          />
        ) : (
          <DockItem key={entry.id} item={entry} onSelect={onSelect} />
        )
      )}
    </nav>
  );
}

/**
 * Detached New Session control — its own glass segment beside the dock strip
 * (OpenDesign `dock-actions` / `dock-new`).
 */
function OsDockNewSession({ onNewSession, className, ...props }: OsDockNewSessionProps) {
  const innerClass = cn(
    "grid size-dock-item place-items-center rounded-dock-item bg-accent text-accent-ink shadow-highlight",
    onNewSession &&
      "transition-transform duration-shell-fast ease-spring hover:-translate-y-0.5 hover:bg-accent-hover"
  );
  const glyph = <Icon as={Plus} className="size-dock-new-icon" />;

  if (!onNewSession) {
    return (
      <div data-slot="os-dock-actions" className={cn(DOCK_SEG, className)}>
        <Tooltip>
          <TooltipTrigger
            render={
              <span
                data-slot="os-dock-new"
                className={innerClass}
                {...(props as React.ComponentProps<"span">)}
              />
            }
          >
            {glyph}
          </TooltipTrigger>
          <DockTip label="New session" />
        </Tooltip>
      </div>
    );
  }

  return (
    <div data-slot="os-dock-actions" className={cn(DOCK_SEG, className)}>
      <Tooltip>
        <TooltipTrigger
          render={
            <button
              type="button"
              data-slot="os-dock-new"
              aria-label="New session"
              className={cn(
                "rounded-dock-item focus-visible:shadow-focus-ring focus-visible:outline-none",
                innerClass
              )}
              onClick={onNewSession}
              {...props}
            />
          }
        >
          {glyph}
        </TooltipTrigger>
        <DockTip label="New session" />
      </Tooltip>
    </div>
  );
}

/**
 * Full dock zone: centered strip + detached New Session, matching OpenDesign
 * `dock-zone` (flex spacers keep the pair centered).
 */
export function OsDockZone({
  items,
  leading,
  onSelect,
  onNewSession,
  magnify = true,
  className,
  style,
  ...props
}: {
  items: OsDockEntry[];
  leading?: React.ReactNode;
  onSelect?: (id: string) => void;
  onNewSession?: () => void;
  magnify?: boolean;
  className?: string;
} & Omit<React.ComponentProps<"div">, "children" | "onSelect">) {
  return (
    <div
      data-slot="os-dock-zone"
      className={cn(
        "pointer-events-none absolute inset-x-0 z-10 flex items-center gap-2.5",
        className
      )}
      style={{
        bottom: "calc(var(--spacing) * 2.5 + env(safe-area-inset-bottom, 0px))",
        paddingInline:
          "calc(var(--spacing) * 4 + max(env(safe-area-inset-left, 0px), env(safe-area-inset-right, 0px)))",
        ...style,
      }}
      {...props}
    >
      <div className="flex min-w-0 flex-1 items-center justify-start overflow-hidden">
        <div className="pointer-events-auto w-fit">{leading}</div>
      </div>
      <div className="flex shrink-0 items-center gap-2.5">
        <OsDock
          items={items}
          onSelect={onSelect}
          magnify={magnify}
          className="pointer-events-auto"
        />
        <OsDockNewSession onNewSession={onNewSession} className="pointer-events-auto" />
      </div>
      <span className="min-w-0 flex-1" aria-hidden="true" />
    </div>
  );
}
