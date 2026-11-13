/**
 * Top-bar desktop indicators (S9). At rest they are the production pager's dots; right-click
 * opens the desktop menu; while a tab is dragged they grow into labelled drop targets.
 */
import { ChevronLeft, ChevronRight, Ellipsis, Pencil, Plus, Trash2 } from "@compozy/ui/icons";

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  cn,
} from "@compozy/ui";

import type { DraftDesktop } from "../window-tabs-shared/model";

export interface DesktopIndicatorsProps {
  desktops: readonly DraftDesktop[];
  activeDesktopId: string;
  /** A tab drag is in progress: indicators are drop targets. */
  targets?: boolean;
  /** The target under the dragged tab. */
  hoverId?: string;
  /** The indicator whose context menu is open. */
  menuId?: string;
  /** Desktops hidden behind the overflow control. */
  hiddenCount?: number;
  /** The dragged tab rests on the overflow control. */
  overflowHover?: boolean;
}

function Dot({ desktop, active }: { desktop: DraftDesktop; active: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "block h-1.5 rounded-pill",
        active ? "w-4.5 bg-fg" : "w-1.5",
        !active && (desktop.needsYou ? "bg-attn" : "bg-indicator")
      )}
    />
  );
}

function IndicatorMenu({ index, count }: { index: number; count: number }) {
  return (
    <DropdownMenuContent align="center" className="min-w-44">
      <DropdownMenuItem>
        <Pencil aria-hidden="true" />
        Rename
      </DropdownMenuItem>
      {index > 0 ? (
        <DropdownMenuItem>
          <ChevronLeft aria-hidden="true" />
          Move left
        </DropdownMenuItem>
      ) : null}
      {index < count - 1 ? (
        <DropdownMenuItem>
          <ChevronRight aria-hidden="true" />
          Move right
        </DropdownMenuItem>
      ) : null}
      {count > 1 ? (
        <DropdownMenuItem variant="destructive">
          <Trash2 aria-hidden="true" />
          Close desktop
        </DropdownMenuItem>
      ) : null}
      <DropdownMenuSeparator />
      <DropdownMenuItem>
        <Plus aria-hidden="true" />
        New desktop
      </DropdownMenuItem>
    </DropdownMenuContent>
  );
}

export function DesktopIndicators({
  desktops,
  activeDesktopId,
  targets = false,
  hoverId,
  menuId,
  hiddenCount = 0,
  overflowHover = false,
}: DesktopIndicatorsProps) {
  return (
    <nav aria-label="Desktops" className="mr-1 flex items-center" data-slot="desktop-indicators">
      <ol className={cn("flex items-center", targets && "gap-1")}>
        {desktops.map((desktop, index) => {
          const active = desktop.id === activeDesktopId;
          const control = targets ? (
            <button
              aria-label={`Move tab to ${desktop.name}`}
              className={cn(
                "flex h-7 items-center gap-1.5 rounded-sm border border-dashed px-2 text-meta font-medium",
                desktop.id === hoverId
                  ? "border-solid border-brand bg-brand-tint-strong text-fg"
                  : "border-line-strong text-fg-2",
                active && desktop.id !== hoverId && "opacity-50"
              )}
              data-slot="desktop-indicator-target"
              type="button"
            >
              <Dot active={active} desktop={desktop} />
              {desktop.name}
            </button>
          ) : (
            <button
              aria-current={active ? "page" : undefined}
              aria-label={desktop.name}
              className={cn(
                "grid h-7.5 w-6 place-items-center rounded-sm hover:bg-rail-hover",
                desktop.id === menuId && "bg-rail-hover"
              )}
              data-slot="desktop-indicator"
              type="button"
            >
              <Dot active={active} desktop={desktop} />
            </button>
          );
          return (
            <li key={desktop.id}>
              {desktop.id === menuId ? (
                <DropdownMenu open>
                  <DropdownMenuTrigger render={control} />
                  <IndicatorMenu count={desktops.length} index={index} />
                </DropdownMenu>
              ) : (
                control
              )}
            </li>
          );
        })}
        {hiddenCount > 0 ? (
          <li>
            <button
              aria-label={`Show ${hiddenCount} more desktops`}
              className={cn(
                "grid h-7.5 place-items-center rounded-sm text-muted-foreground hover:bg-rail-hover",
                targets ? "border border-dashed border-line-strong px-2" : "w-6",
                overflowHover && "border-solid border-brand bg-brand-tint-strong text-fg"
              )}
              data-slot="desktop-indicator-overflow"
              type="button"
            >
              {targets ? (
                <span className="text-meta font-medium">+{hiddenCount}</span>
              ) : (
                <Ellipsis aria-hidden="true" className="size-3.5" />
              )}
            </button>
          </li>
        ) : null}
      </ol>
    </nav>
  );
}
