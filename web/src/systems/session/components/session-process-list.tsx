import { CircleStop } from "@compozy/ui/icons";

import { Button, Disclosure, StateGlyph, Time } from "@compozy/ui";

import { ComposerPopoverRowsSkeleton } from "./composer-popover-rows-skeleton";
import {
  processCommandLabel,
  processIdentity,
  processHelperCountLabel,
  processHelperGroups,
  PROCESS_LINEAGE_HINT,
  PROCESS_LINEAGE_LABEL,
  processKindLabel,
  processLinkedByLineage,
  processPortHref,
  type SessionProcessTree,
} from "../lib/session-processes";

export interface SessionProcessListProps {
  /** One row per logical process tree. */
  trees: readonly SessionProcessTree[];
  /** False when the daemon cannot scan processes on this system. */
  supported?: boolean;
  loading?: boolean;
  failed?: boolean;
  /** The list read found no such session; shown without a retry. */
  missing?: boolean;
  onRetry?: () => void;
  /** Stops the whole tree: every member the daemon still verifies. */
  onStop?: (tree: SessionProcessTree) => void;
  /**
   * Lead identities whose stop is in flight (they show the spinner), or `"all"`. Any
   * stop in flight locks every row: one stop at a time, so a second request can
   * never hide the first one's pending state.
   */
  stopping?: ReadonlySet<string> | "all" | null;
  /** How many rows the list expects before its first read (capped). */
  placeholderCount?: number;
  /** The line once nothing runs. */
  emptyLabel?: string;
  /** Trees whose helpers start unfolded (prototypes and stories). */
  defaultExpanded?: readonly string[];
  /**
   * Set while the owner offers Stop all: tool-call trees then sit under a caption saying
   * Stop all leaves them to their turn.
   */
  stopAllExcludesTools?: boolean;
}

// The row's geometry, shared by the live rows and their placeholders so the two cannot drift.
const PROCESS_ROW_FRAME = "flex items-start gap-2 rounded-md px-0.5 py-1";
const PROCESS_ROW_TEXT_FRAME = "flex min-w-0 flex-1 flex-col gap-0.5";

const COMMAND_WIDTHS = ["w-52", "w-40", "w-60"] as const;
const META_WIDTHS = ["w-32", "w-24", "w-36"] as const;

function ProcessPorts({ ports }: { ports: readonly number[] }) {
  if (ports.length === 0) return null;
  return (
    <span className="flex min-w-0 flex-wrap items-center gap-x-2">
      {ports.map(port => (
        <a
          key={port}
          href={processPortHref(port)}
          target="_blank"
          rel="noreferrer"
          className="font-mono text-mono-id text-info hover:underline hover:underline-offset-2"
        >
          localhost:{port}
        </a>
      ))}
    </span>
  );
}

function ProcessHelpers({ tree, defaultOpen }: { tree: SessionProcessTree; defaultOpen: boolean }) {
  if (tree.helpers.length === 0) return null;
  return (
    <Disclosure
      label={processHelperCountLabel(tree.helpers.length)}
      defaultOpen={defaultOpen}
      triggerProps={{ className: "text-transcript-caption", "data-testid": "process-helpers" }}
    >
      <ul className="flex flex-col gap-0.5 border-l border-line pt-0.5 pl-2">
        {processHelperGroups(tree.helpers).map(group => (
          <li
            key={group.program}
            className="flex min-w-0 items-baseline gap-1.5 font-mono text-mono-id text-muted"
            title={group.title}
          >
            <span className="truncate">{group.program}</span>
            {group.count > 1 ? (
              <span className="shrink-0 text-faint tabular-nums">×{group.count}</span>
            ) : null}
          </li>
        ))}
      </ul>
    </Disclosure>
  );
}

function stopLabel(tree: SessionProcessTree, command: string) {
  const others = tree.wrappers.length + tree.helpers.length;
  if (others === 0) return `Stop process: ${command}`;
  return `Stop process: ${command} and ${others} more in its tree`;
}

function ProcessRow({
  tree,
  stopping,
  locked,
  expanded,
  onStop,
}: {
  tree: SessionProcessTree;
  stopping: boolean;
  locked: boolean;
  expanded: boolean;
  onStop?: (tree: SessionProcessTree) => void;
}) {
  const { lead } = tree;
  const command = processCommandLabel(lead);
  const wrapper = tree.wrappers[0];
  const chain = [...tree.wrappers, lead].map(member => member.command).join("\n→ ");
  return (
    <li
      className={PROCESS_ROW_FRAME}
      data-testid={`session-process-${lead.pid}`}
      data-kind={tree.kind}
    >
      <StateGlyph size="sm" state="running" className="mt-0.75" />
      <span className={PROCESS_ROW_TEXT_FRAME}>
        <span
          className="line-clamp-2 font-mono text-transcript-meta break-all text-fg"
          title={lead.cwd ? `${chain}\n${lead.cwd}` : chain}
        >
          {command}
        </span>
        <span className="flex min-w-0 flex-wrap items-center gap-x-1 text-transcript-caption text-muted">
          <span className="shrink-0">{processKindLabel(tree.kind)}</span>
          <span aria-hidden="true">·</span>
          <span className="shrink-0">
            started <Time iso={(wrapper ?? lead).started_at} />
          </span>
          {processLinkedByLineage(lead) ? (
            <>
              <span aria-hidden="true">·</span>
              <span className="shrink-0 text-faint" title={PROCESS_LINEAGE_HINT}>
                {PROCESS_LINEAGE_LABEL}
              </span>
            </>
          ) : null}
        </span>
        {wrapper ? (
          <span
            className="truncate text-transcript-caption text-faint"
            data-testid="process-wrapper"
          >
            via <span className="font-mono text-mono-id">{processCommandLabel(wrapper)}</span>
          </span>
        ) : null}
        <ProcessPorts ports={tree.ports} />
        <ProcessHelpers tree={tree} defaultOpen={expanded} />
      </span>
      {onStop ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={stopLabel(tree, command)}
          disabled={stopping || locked}
          onClick={() => onStop(tree)}
          className="text-muted"
        >
          {stopping ? (
            <StateGlyph size="sm" state="running" />
          ) : (
            <CircleStop aria-hidden="true" className="size-3.5" data-slot="process-stop-icon" />
          )}
        </Button>
      ) : null}
    </li>
  );
}

export function ProcessListNote({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <div className="flex items-center justify-between gap-2 px-0.5">
      <p className="text-transcript-caption text-muted">{message}</p>
      {onRetry ? (
        <Button type="button" variant="ghost" size="xs" onClick={onRetry}>
          Try again
        </Button>
      ) : null}
    </div>
  );
}

/**
 * Process rows with every read state: placeholders before the first read, the
 * rows (cached rows stay when a refresh fails), an error with a retry, and a
 * plain empty line once nothing runs.
 */
export function SessionProcessList({
  trees,
  supported = true,
  loading = false,
  failed = false,
  missing = false,
  onRetry,
  onStop,
  stopping = null,
  placeholderCount = 1,
  emptyLabel = "No processes are running.",
  defaultExpanded = [],
  stopAllExcludesTools = false,
}: SessionProcessListProps) {
  // A gone session outranks rows cached from an earlier read.
  if (missing) return <ProcessListNote message="This session is no longer available." />;
  if (!supported) {
    return <ProcessListNote message="Processes can't be listed on this system." />;
  }
  if (trees.length > 0) {
    const row = (tree: SessionProcessTree) => (
      <ProcessRow
        key={tree.id}
        tree={tree}
        stopping={
          stopping !== null && stopping !== "all" && stopping.has(processIdentity(tree.lead))
        }
        locked={stopping !== null}
        expanded={defaultExpanded.includes(tree.id)}
        onStop={onStop}
      />
    );
    const left = stopAllExcludesTools ? trees.filter(tree => tree.kind !== "tool") : trees;
    const turn = stopAllExcludesTools ? trees.filter(tree => tree.kind === "tool") : [];
    return (
      <div className="flex flex-col gap-1">
        <div className="flex max-h-80 flex-col gap-1 overflow-y-auto">
          {left.length > 0 ? <ul className="flex flex-col gap-0.5">{left.map(row)}</ul> : null}
          {turn.length > 0 ? (
            <section aria-label="Running in this turn" className="flex flex-col gap-0.5">
              <p className="px-0.5 pt-1 text-transcript-caption text-faint">
                Running in this turn · Stop all leaves these to the agent
              </p>
              <ul className="flex flex-col gap-0.5">{turn.map(row)}</ul>
            </section>
          ) : null}
        </div>
        {failed ? (
          <ProcessListNote message="Couldn't refresh processes." onRetry={onRetry} />
        ) : null}
      </div>
    );
  }
  if (failed) return <ProcessListNote message="Couldn't load processes." onRetry={onRetry} />;
  if (loading) {
    return (
      <ComposerPopoverRowsSkeleton
        label="Loading processes"
        data-testid="session-processes-loading"
        count={placeholderCount}
        rowClassName={PROCESS_ROW_FRAME}
        textClassName={PROCESS_ROW_TEXT_FRAME}
        titleWidths={COMMAND_WIDTHS}
        metaWidths={META_WIDTHS}
      />
    );
  }
  return <p className="px-0.5 text-transcript-caption text-muted">{emptyLabel}</p>;
}
