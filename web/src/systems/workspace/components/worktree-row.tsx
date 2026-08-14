import { FolderGit2, FolderSearch, FolderX, Loader } from "lucide-react";
import type { LucideIcon } from "lucide-react";

import { cn, Icon, MonoId, PillDot, type PillTone } from "@compozy/ui";

import type { WorktreeNestEntry } from "../lib/worktree-display";
import type { WorktreeDisplayState } from "../types";
import { WorktreePath } from "./worktree-path";
import { WorktreeDetachedPin } from "./worktree-signal-parts";
import { WorktreeSignals } from "./worktree-signals";
import { WorktreeStateChip } from "./worktree-state-chip";

const STATE_ICON: Record<WorktreeDisplayState, LucideIcon> = {
  ready: FolderGit2,
  pending: Loader,
  discovered: FolderSearch,
  missing: FolderX,
  error: FolderGit2,
  failed: FolderX,
};

/** Discovered and missing wells are dashed: nothing of ours is in that directory. */
const DASHED_WELL_STATES = new Set<WorktreeDisplayState>(["discovered", "missing"]);

/** Nest rows compress state into a dot; the chip still carries the word. */
const STATE_DOT_TONE: Record<WorktreeDisplayState, PillTone> = {
  ready: "success",
  pending: "warning",
  discovered: "info",
  missing: "neutral",
  error: "danger",
  failed: "danger",
};

export interface WorktreeRowProps extends Omit<React.ComponentProps<"div">, "onSelect"> {
  entry: WorktreeNestEntry;
  /** Full row, or the compact density used inside worktree submenus. */
  density?: "full" | "nest";
  selected?: boolean;
  /** Detached HEAD replaces the branch lane with the pin (US-012 EC-1). */
  detachedSha?: string | null;
  staleLabel?: string | null;
  merged?: boolean | null;
  /** Trailing affordance — Adopt, Resolve…, or a relative timestamp. */
  trailing?: React.ReactNode;
}

/**
 * The shared row vocabulary. The name is the record label, never the path
 * basename: two worktrees can sit in identically named directories, and the
 * label is what the user chose.
 */
export function WorktreeRow({
  entry,
  density = "full",
  selected,
  detachedSha,
  staleLabel,
  merged,
  trailing,
  className,
  ...props
}: WorktreeRowProps) {
  const isNest = density === "nest";
  const worktree = entry.worktree;
  const inert = entry.inertReason !== null;

  return (
    <div
      data-slot="worktree-row"
      data-state={entry.displayState}
      data-density={density}
      data-selected={selected ? "true" : undefined}
      data-inert={inert ? "true" : undefined}
      className={cn(
        "grid min-w-0 grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-2.5 rounded-md px-2.5",
        isNest ? "py-1" : "min-h-11 py-[7px]",
        inert ? "cursor-default" : "hover:bg-row-hover",
        selected && "bg-row-selected",
        className
      )}
      {...props}
    >
      {isNest ? (
        <PillDot
          data-slot="worktree-row-dot"
          data-state={entry.displayState}
          tone={STATE_DOT_TONE[entry.displayState]}
          size="sm"
        />
      ) : (
        <span
          aria-hidden="true"
          data-slot="worktree-row-icon"
          className={cn(
            "grid size-[26px] shrink-0 place-items-center rounded",
            DASHED_WELL_STATES.has(entry.displayState)
              ? "border border-dashed border-line-strong text-muted"
              : "bg-elevated text-muted"
          )}
        >
          <Icon as={STATE_ICON[entry.displayState]} size="sm" />
        </span>
      )}

      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="flex min-w-0 items-center gap-2">
          <b
            data-slot="worktree-row-name"
            className={cn(
              "min-w-0 truncate text-small-body font-[550] tracking-tight",
              entry.displayState === "missing" ||
                entry.displayState === "failed" ||
                entry.displayState === "error"
                ? "text-muted"
                : "text-fg-strong"
            )}
          >
            {entry.name}
          </b>
          {entry.displayState === "ready" ? (
            // The nest dot already says "ready"; a second dot beside the name
            // would repeat the same signal.
            isNest ? null : (
              <PillDot tone="success" size="sm" />
            )
          ) : // Nested rows carry the chip on the path line, small and quiet.
          isNest ? null : (
            <WorktreeStateChip state={entry.displayState} />
          )}
          {isNest ? null : (
            <WorktreeSignals
              dirty={worktree?.dirty ?? null}
              ahead={worktree?.ahead ?? null}
              behind={worktree?.behind ?? null}
              agentActivity={worktree?.agent_activity ?? "idle"}
              origin={worktree?.origin ?? ""}
              setupState={worktree?.setup_state ?? "none"}
              setupError={worktree?.setup_error}
              merged={merged}
              staleLabel={staleLabel}
            />
          )}
        </span>

        {isNest ? (
          <span className="flex min-w-0 items-center gap-1.5 overflow-hidden">
            <WorktreePath className="min-w-0 flex-1" focusable={false} path={entry.path} />
            <WorktreeStateChip className="shrink-0" state={entry.displayState} size="sm" />
          </span>
        ) : (
          <span className="flex min-w-0 items-center gap-2 overflow-hidden">
            {detachedSha ? (
              <WorktreeDetachedPin sha={detachedSha} />
            ) : (
              <MonoId value={entry.branch} copy preserveCase copyLabel="Copy branch" />
            )}
            <WorktreePath className="min-w-0 flex-1" focusable={false} path={entry.path} />
          </span>
        )}
      </span>

      <span className="flex shrink-0 items-center gap-1.5">
        {isNest ? (
          <WorktreeSignals
            density="nest"
            dirty={worktree?.dirty ?? null}
            ahead={worktree?.ahead ?? null}
            behind={worktree?.behind ?? null}
            agentActivity={worktree?.agent_activity ?? "idle"}
            origin={worktree?.origin ?? ""}
            setupState={worktree?.setup_state ?? "none"}
            setupError={worktree?.setup_error}
            merged={merged}
          />
        ) : null}
        {entry.inertReason ? (
          // Functional label in its own lane — never hover-only, because the
          // reason is what stops the user from trying again.
          <span data-slot="worktree-inert-reason" className="font-mono text-micro text-faint">
            {entry.inertReason}
          </span>
        ) : null}
        {trailing}
      </span>
    </div>
  );
}
