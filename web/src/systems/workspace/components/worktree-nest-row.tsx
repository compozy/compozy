import { Check, Circle, CircleCheck } from "@compozy/ui/icons";

import { cn } from "@compozy/ui";

import { contractHomePath } from "../lib/display-path";
import type { WorktreeNestEntry } from "../lib/worktree-display";
import type { WorktreeDisplayState } from "../types";
import { WorktreePath } from "./worktree-path";
import { WorktreeSignals } from "./worktree-signals";
import { WorktreeStateDot } from "./worktree-state-dot";
import { ownerFromRow, ProfileOwnerTag } from "@/systems/profiles";

/**
 * Plain-language state words for the nest's second line. Ready says nothing —
 * the name alone is the calm read — and every other state is neutral text, so
 * colour stays reserved for the leading glyph of states that need attention.
 */
const NEST_STATE_LABEL: Record<Exclude<WorktreeDisplayState, "ready">, string> = {
  pending: "Setting up",
  discovered: "Discovered",
  missing: "Missing",
  error: "Unreadable",
  failed: "Failed",
};

function NestRowLeading({ entry }: { entry: WorktreeNestEntry }) {
  if (entry.displayState === "ready") {
    return (
      <span
        aria-hidden="true"
        data-slot="worktree-state-dot"
        data-state="ready"
        className="inline-block size-status-dot shrink-0 rounded-full bg-line-strong"
      />
    );
  }
  return <WorktreeStateDot state={entry.displayState} />;
}

/** Selection mark in the leading lane, so it never reads as the trailing scope check. */
function NestRowSelectMark({ selected, disabled }: { selected: boolean; disabled: boolean }) {
  const Mark = selected ? CircleCheck : Circle;
  return (
    <Mark
      aria-hidden="true"
      data-slot="worktree-nest-select-mark"
      data-selected={selected ? "true" : undefined}
      className={cn(
        "size-3.5 shrink-0",
        selected ? "text-fg-strong" : "text-faint",
        disabled && "opacity-50"
      )}
    />
  );
}

/**
 * Second line: state word, owner glyph, then the path — or, when the row
 * cannot act (a discovered checkout Git cannot use, a refused selection), the
 * reason in place of the path.
 */
function NestRowMeta({
  entry,
  userHomeDir,
  refusal,
}: {
  entry: WorktreeNestEntry;
  userHomeDir?: string;
  refusal?: string | null;
}) {
  const label = entry.displayState === "ready" ? null : NEST_STATE_LABEL[entry.displayState];
  const gitReason = refusal || (entry.discovered && !entry.selectable ? entry.inertReason : null);
  return (
    <span className="flex min-w-0 items-center gap-1.5 text-micro text-subtle">
      {label ? (
        <span data-slot="worktree-nest-state" className="shrink-0">
          {label}
        </span>
      ) : null}
      {label ? (
        <span aria-hidden="true" className="shrink-0 text-faint">
          ·
        </span>
      ) : null}
      {/* Owner-tagged in every profile, like the full-density row; glyph-only at nest density. */}
      {entry.worktree ? (
        <ProfileOwnerTag compact size="xs" owner={ownerFromRow(entry.worktree)} />
      ) : null}
      {gitReason ? (
        <span data-slot="worktree-nest-reason" title={gitReason} className="min-w-0 truncate">
          {gitReason}
        </span>
      ) : (
        <WorktreePath
          focusable={false}
          path={entry.path}
          userHomeDir={userHomeDir}
          className="text-mono-id text-faint tabular-nums"
        />
      )}
    </span>
  );
}

function NestRowTrailing({ entry, checked }: { entry: WorktreeNestEntry; checked: boolean }) {
  const worktree = entry.worktree;
  return (
    <>
      {entry.adoptable ? (
        <span
          className={cn(
            "text-micro font-medium text-muted opacity-0 transition-opacity duration-base",
            "group-hover/wtnest:opacity-100 group-focus-within/wtnest:opacity-100",
            "group-data-[on=true]/wtnest:opacity-100"
          )}
        >
          Adopt
        </span>
      ) : entry.inertReason ? null : (
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
        <Check aria-hidden="true" className="size-deck-glyph shrink-0 text-fg" />
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
  /**
   * Bulk-selection mode: the leading lane carries the selection mark instead of
   * the state glyph, and the trailing scope check is withheld.
   */
  selection?: { selected: boolean; reason?: string | null };
  /** Renders the sr-only state/path/reason description under this id. */
  descriptionId?: string;
}

/**
 * The one two-line nest row every host renders: a neutral leading glyph (colour
 * only for states needing attention), the name, then one quiet metadata line,
 * with a single trailing signal and the scope check. Hosts wrap it in their
 * own interactive chrome under a `group/wtnest` class (`data-on` while focused)
 * for the reveal states.
 */
export function WorktreeNestRow({
  entry,
  userHomeDir,
  checked = false,
  selection,
  descriptionId,
  className,
  ...props
}: WorktreeNestRowProps) {
  const inert = !entry.selectable;
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
      <span className="flex justify-center self-start pt-1">
        {selection ? (
          <NestRowSelectMark selected={selection.selected} disabled={Boolean(selection.reason)} />
        ) : (
          <NestRowLeading entry={entry} />
        )}
      </span>
      <span className="flex min-w-0 flex-col gap-0.5">
        <b
          data-slot="worktree-nest-row-name"
          className={cn(
            "min-w-0 truncate text-small-body font-medium tracking-tight",
            inert ? "text-muted" : "text-fg"
          )}
        >
          {entry.name}
        </b>
        <NestRowMeta entry={entry} userHomeDir={userHomeDir} refusal={selection?.reason} />
      </span>
      <span className="flex flex-none items-center gap-workspaces-tile-gap pr-1">
        <NestRowTrailing entry={entry} checked={!selection && checked} />
      </span>
    </span>
  );
}
