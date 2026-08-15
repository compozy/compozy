import { Check } from "lucide-react";

import { cn } from "@compozy/ui";

import { contractHomePath } from "../lib/display-path";
import type { WorktreeNestEntry } from "../lib/worktree-display";
import { WorktreePath } from "./worktree-path";
import { WorktreeSignals } from "./worktree-signals";
import { WorktreeStateChip } from "./worktree-state-chip";
import { WorktreeStateDot } from "./worktree-state-dot";

function NestRowDot({ entry }: { entry: WorktreeNestEntry }) {
  if (entry.displayState === "ready") {
    return (
      <span
        aria-hidden="true"
        data-slot="worktree-state-dot"
        data-state="ready"
        className="inline-block size-status-dot shrink-0 rounded-full bg-success"
      />
    );
  }
  return <WorktreeStateDot state={entry.displayState} />;
}

function NestRowTrailing({ entry, checked }: { entry: WorktreeNestEntry; checked: boolean }) {
  const worktree = entry.worktree;
  return (
    <>
      {entry.inertReason ? (
        // Functional reason in its own capped lane — never hover-only; long
        // runtime reasons truncate and the full text stays in the description.
        <span
          data-slot="worktree-nest-reason"
          title={entry.inertReason}
          className="max-w-worktree-nest-reason truncate font-mono text-micro text-faint"
        >
          {entry.inertReason}
        </span>
      ) : entry.adoptable ? (
        <span
          className={cn(
            "text-mono-id font-semibold text-info opacity-0 transition-opacity duration-base",
            "group-hover/wtnest:opacity-100 group-focus-within/wtnest:opacity-100",
            "group-data-[on=true]/wtnest:opacity-100"
          )}
        >
          Adopt
        </span>
      ) : (
        <WorktreeSignals
          density="nest"
          dirty={worktree?.dirty ?? null}
          ahead={worktree?.ahead ?? null}
          behind={worktree?.behind ?? null}
          agentActivity={worktree?.agent_activity ?? "idle"}
          origin={worktree?.origin ?? ""}
          setupState={worktree?.setup_state ?? "none"}
          setupError={worktree?.setup_error}
        />
      )}
      {checked ? (
        // One selection marker per layer: the check lives on the nest row
        // while the parent surface carries its own scope marker.
        <Check aria-hidden="true" className="size-deck-glyph shrink-0 text-fg-strong" />
      ) : null}
    </>
  );
}

export interface WorktreeNestRowProps extends Omit<React.ComponentProps<"span">, "children"> {
  entry: WorktreeNestEntry;
  /** Display-contracts home-rooted paths; actions keep the absolute path. */
  userHomeDir?: string;
  /** This row is the bound worktree scope — renders the trailing check. */
  checked?: boolean;
  /** Renders the sr-only state/path/reason description under this id. */
  descriptionId?: string;
}

/**
 * The one two-line nest row every host renders: state dot, name, then chip +
 * path, with a single trailing signal (reason · Adopt · live signals) and the
 * scope check. Hosts wrap it in their own interactive chrome under a
 * `group/wtnest` class (`data-on` while focused) for the reveal states.
 */
export function WorktreeNestRow({
  entry,
  userHomeDir,
  checked = false,
  descriptionId,
  className,
  ...props
}: WorktreeNestRowProps) {
  return (
    <span
      data-slot="worktree-nest-row-body"
      className={cn(
        "grid min-w-0 grid-cols-[16px_minmax(0,1fr)_auto] items-center gap-2",
        className
      )}
      {...props}
    >
      {descriptionId ? (
        <span id={descriptionId} className="sr-only">
          {entry.displayState}. {contractHomePath(entry.path, userHomeDir)}.{" "}
          {entry.inertReason ?? ""}
        </span>
      ) : null}
      <span className="mt-1 flex justify-center self-start">
        <NestRowDot entry={entry} />
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <b
          data-slot="worktree-nest-row-name"
          className={cn(
            "min-w-0 truncate text-small-body font-medium tracking-tight",
            entry.displayState === "missing" ? "text-muted" : "text-fg-strong"
          )}
        >
          {entry.name}
        </b>
        <span className="flex min-w-0 items-center gap-1.5">
          {entry.displayState === "ready" ? null : (
            <WorktreeStateChip className="h-3.5" size="sm" state={entry.displayState} />
          )}
          <WorktreePath
            focusable={false}
            path={entry.path}
            userHomeDir={userHomeDir}
            className="text-mono-id tabular-nums"
          />
        </span>
      </span>
      <span className="flex flex-none items-center gap-workspaces-tile-gap pr-1">
        <NestRowTrailing entry={entry} checked={checked} />
      </span>
    </span>
  );
}
