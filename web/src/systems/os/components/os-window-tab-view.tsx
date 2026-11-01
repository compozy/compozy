import { Pill, Tooltip, TooltipContent, TooltipTrigger, type TopbarSlotValue } from "@compozy/ui";
import { X } from "lucide-react";
import * as React from "react";

import { cn } from "@/lib/utils";

export interface OsWindowTabViewProps {
  active: boolean;
  crumbs?: TopbarSlotValue["crumbs"];
  glyph: React.ReactNode;
  isNewTab?: boolean;
  label: React.ReactNode;
  onActivate?: () => void;
  onClose?: (event: React.MouseEvent) => void;
  onMiddleClose?: () => void;
  onTabPointerDown?: (event: React.PointerEvent<HTMLElement>) => void;
  pendingCount?: number;
  pinned?: boolean;
  title: React.ReactNode;
  windowId: string;
}

/** A deck tab from resolved facts; `OsWindowTab` binds the window, its session and its slot. */
export function OsWindowTabView({
  active,
  crumbs,
  glyph,
  isNewTab = false,
  label,
  onActivate,
  onClose,
  onMiddleClose,
  onTabPointerDown,
  pendingCount = 0,
  pinned = false,
  title,
  windowId,
}: OsWindowTabViewProps) {
  const showLabel = !pinned;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div
            data-slot="os-window-tab"
            data-window-id={windowId}
            data-active={active ? "" : undefined}
            data-pinned={pinned ? "" : undefined}
            data-testid={`os-window-tab-${windowId}`}
            className={tabPlateClass(pinned, active)}
          >
            {active ? <OsWindowTabFeet /> : null}
            <button
              type="button"
              role="tab"
              aria-selected={active}
              data-slot="os-window-tab-activate"
              className="flex h-full min-w-0 flex-1 cursor-pointer items-center gap-2 self-stretch rounded-[inherit] pl-3 text-left focus-visible:outline-none focus-visible:shadow-focus-inset"
              onPointerDown={event => {
                if (event.button === 0) onTabPointerDown?.(event);
              }}
              onClick={onActivate}
              onAuxClick={event => {
                if (event.button === 1 && !pinned) {
                  event.preventDefault();
                  onMiddleClose?.();
                }
              }}
            >
              {glyph}
              {showLabel ? (
                <span className={cn("min-w-0 flex-1 truncate", isNewTab && "text-subtle")}>
                  {label}
                </span>
              ) : null}
              <Pill.Count count={pendingCount} data-slot="os-window-tab-count" />
            </button>
            {showLabel ? (
              <OsWindowTabClose active={active} label={label} onClick={onClose} />
            ) : null}
          </div>
        }
      />
      <TooltipContent side="bottom">
        <OsWindowTabTrail crumbs={crumbs} title={title} />
      </TooltipContent>
    </Tooltip>
  );
}

function tabPlateClass(pinned: boolean, active: boolean): string {
  return cn(
    "group/tab relative inline-flex h-deck-tab min-w-0 items-center gap-2 rounded-t-deck-tab text-body font-medium transition-colors duration-base select-none",
    pinned ? "shrink-0 pr-3" : "flex-1 pr-1.75",
    active
      ? // The plate's side and top hairlines are inset so the feet meet them flush.
        "z-1 bg-canvas text-fg shadow-[inset_1px_0_0_var(--color-line),inset_-1px_0_0_var(--color-line),inset_0_1px_0_var(--color-line)]"
      : "text-muted hover:bg-rail-hover hover:text-fg"
  );
}

/** The tooltip's full trail: every drill-in crumb, then the tab's title. */
function OsWindowTabTrail({
  crumbs,
  title,
}: {
  crumbs: TopbarSlotValue["crumbs"] | undefined;
  title: React.ReactNode;
}) {
  return (
    <span className="flex max-w-64 min-w-0 items-center gap-1">
      {(crumbs ?? []).map(part => (
        <React.Fragment key={part.id}>
          <span className="truncate">{part.label}</span>
          <span aria-hidden="true">/</span>
        </React.Fragment>
      ))}
      <span className="truncate">{title}</span>
    </span>
  );
}

/**
 * Concave feet (DESIGN-NOTES round 3): a 10px box per side whose fully
 * rounded bottom corner (clamped to the box, so a 10px radius) draws the arc from the tab's side hairline into the strip's
 * bottom hairline, overlapping the tab edge by 1px; its canvas box-shadow
 * fills the plate side of the arc so there is no seam.
 */
function OsWindowTabFeet() {
  const foot =
    "pointer-events-none absolute bottom-0 size-2.5 border-0 border-b border-solid border-line";
  return (
    <>
      <span
        aria-hidden="true"
        data-slot="os-window-tab-foot"
        className={cn(
          foot,
          "-left-2.25 rounded-br-full border-r shadow-[5px_5px_0_5px_var(--color-canvas)]"
        )}
      />
      <span
        aria-hidden="true"
        data-slot="os-window-tab-foot"
        className={cn(
          foot,
          "-right-2.25 rounded-bl-full border-l shadow-[-5px_5px_0_5px_var(--color-canvas)]"
        )}
      />
    </>
  );
}

function OsWindowTabClose({
  active,
  label,
  onClick,
}: {
  active: boolean;
  label: React.ReactNode;
  onClick?: (event: React.MouseEvent) => void;
}) {
  return (
    <button
      type="button"
      aria-label={`Close ${typeof label === "string" ? label : "tab"}`}
      data-slot="os-window-tab-close"
      className={cn(
        "grid size-deck-close shrink-0 place-items-center rounded-pill text-subtle opacity-0 transition-opacity duration-base",
        "hover:bg-selected hover:text-fg focus-visible:opacity-100 focus-visible:shadow-focus-ring focus-visible:outline-none",
        "group-hover/tab:opacity-100",
        active && "opacity-100"
      )}
      onPointerDown={event => event.stopPropagation()}
      onClick={onClick}
    >
      <X aria-hidden="true" className="size-3" />
    </button>
  );
}
