import { Check, GitBranch, Plus } from "lucide-react";

import { cn } from "@compozy/ui";

/** `root` = check pill; `wt` = branch badge while a worktree is the scope. */
export type OsWorkspaceTileCurrent = "root" | "wt";

/*
 * A11y contract deviation, authorized by the workspaces DESIGN-NOTES: the
 * frosted plate (`data-on`) is the SOLE focus indicator on tiles — no
 * `--shadow-focus-ring`, no border, no flat fill, and the name never grays.
 * Only the default outline is suppressed; the plate itself renders on every
 * roving-focus move, so keyboard focus is always visible.
 */
const TILE_CLASS = cn(
  "group/wsov-tile relative flex w-[94px] flex-none snap-center flex-col items-center gap-[7px]",
  "rounded-xl border border-transparent px-1.5 pt-2 pb-[7px] text-center select-none",
  "min-[960px]:w-[86px]",
  "outline-none focus-visible:outline-none",
  "transition-colors duration-base ease-out",
  "data-[on=true]:bg-(image:--shell-plate-gradient) data-[on=true]:shadow-shell-plate",
  "data-[on=true]:backdrop-blur-shell-plate data-[on=true]:backdrop-saturate-[1.2]"
);

const WELL_CLASS = cn(
  "relative grid size-14 flex-none place-items-center rounded-window text-fg",
  "border border-line-strong bg-(image:--shell-well-gradient) shadow-highlight",
  "min-[960px]:size-[52px]",
  "transition-[background,border-color,box-shadow,transform,scale] duration-base ease-out",
  "group-hover/wsov-tile:bg-none group-hover/wsov-tile:bg-btn-default-hover",
  "group-data-[on=true]/wsov-tile:-translate-y-px",
  "group-active/wsov-tile:scale-[0.96] group-active/wsov-tile:translate-y-0"
);

const NAME_CLASS =
  "block w-full truncate text-form-label leading-tight font-medium tracking-tight text-fg-strong";

function CurrentPill({ current }: { current: OsWorkspaceTileCurrent }) {
  const Glyph = current === "wt" ? GitBranch : Check;
  return (
    <span
      aria-hidden="true"
      data-slot="os-workspace-tile-current"
      className="absolute -right-1 -bottom-1 grid size-4 place-items-center rounded-pill bg-success text-accent-ink shadow-shell-check-halo"
    >
      <Glyph className="size-2.5" />
    </span>
  );
}

export interface OsWorkspaceTileProps extends Omit<
  React.ComponentProps<"button">,
  "children" | "name"
> {
  name: string;
  monogram: string;
  focused: boolean;
  /** Absent while Global scope is on — no tile is current then. */
  current?: OsWorkspaceTileCurrent | null;
}

/**
 * Compact Command-Tab tile: 52px monogram well + name. Current is the success
 * pill on the well (branch glyph while a worktree is the scope); focus is the
 * frosted plate behind the whole tile.
 */
export function OsWorkspaceTile({
  name,
  monogram,
  focused,
  current = null,
  className,
  ...props
}: OsWorkspaceTileProps) {
  return (
    <button
      type="button"
      role="option"
      data-slot="os-workspace-tile"
      data-on={focused ? "true" : undefined}
      data-current={current ?? undefined}
      aria-selected={focused}
      aria-label={current ? `${name} (current)` : name}
      tabIndex={focused ? 0 : -1}
      className={cn(TILE_CLASS, className)}
      {...props}
    >
      <span aria-hidden="true" className={WELL_CLASS}>
        <span className="font-mono text-card-title font-semibold tracking-mono tabular-nums">
          {monogram}
        </span>
        {current ? <CurrentPill current={current} /> : null}
      </span>
      <span className={NAME_CLASS}>{name}</span>
    </button>
  );
}

export interface OsWorkspaceAddTileProps extends Omit<React.ComponentProps<"button">, "children"> {
  focused: boolean;
}

/** Trailing dashed "New" tile — secondary by contract, never accent-filled. */
export function OsWorkspaceAddTile({ focused, className, ...props }: OsWorkspaceAddTileProps) {
  return (
    <button
      type="button"
      role="option"
      data-slot="os-workspace-tile"
      data-add="true"
      data-on={focused ? "true" : undefined}
      aria-selected={focused}
      aria-label="New workspace"
      tabIndex={focused ? 0 : -1}
      className={cn(TILE_CLASS, className)}
      {...props}
    >
      <span
        aria-hidden="true"
        className={cn(
          WELL_CLASS,
          "border-dashed bg-none text-muted shadow-none",
          "group-hover/wsov-tile:text-fg-strong group-hover/wsov-tile:bg-btn-default-fill",
          "group-data-[on=true]/wsov-tile:text-fg-strong"
        )}
      >
        <Plus className="size-4.5" />
      </span>
      <span className={cn(NAME_CLASS, "text-muted", "group-hover/wsov-tile:text-fg-strong")}>
        New
      </span>
    </button>
  );
}
