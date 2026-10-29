import { CircleStop } from "lucide-react";

import { Button, StateGlyph, Time } from "@compozy/ui";

import type { SessionProcessPayload } from "../adapters/session-processes-api";
import { ComposerPopoverRowsSkeleton } from "./composer-popover-rows-skeleton";
import {
  processCommandLabel,
  processIdentity,
  PROCESS_LINEAGE_HINT,
  PROCESS_LINEAGE_LABEL,
  processKindLabel,
  processLinkedByLineage,
  processPortHref,
} from "../lib/session-processes";

export interface SessionProcessListProps {
  processes: readonly SessionProcessPayload[];
  /** False when the daemon cannot scan processes on this system. */
  supported?: boolean;
  loading?: boolean;
  failed?: boolean;
  /** The list read found no such session; shown without a retry. */
  missing?: boolean;
  onRetry?: () => void;
  onStop?: (process: SessionProcessPayload) => void;
  /** Identities whose stop is in flight; `"all"` locks every row. */
  stopping?: ReadonlySet<string> | "all" | null;
  /** How many rows the list expects before its first read (capped). */
  placeholderCount?: number;
  /** The line once nothing runs. */
  emptyLabel?: string;
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

function ProcessRow({
  process,
  stopping,
  locked,
  onStop,
}: {
  process: SessionProcessPayload;
  stopping: boolean;
  locked: boolean;
  onStop?: (process: SessionProcessPayload) => void;
}) {
  const command = processCommandLabel(process);
  return (
    <li
      className={PROCESS_ROW_FRAME}
      data-testid={`session-process-${process.pid}`}
      data-kind={process.kind}
    >
      <StateGlyph size="sm" state="running" className="mt-0.75" />
      <span className={PROCESS_ROW_TEXT_FRAME}>
        <span
          className="line-clamp-2 font-mono text-transcript-meta break-all text-fg"
          title={process.cwd ? `${command}\n${process.cwd}` : command}
        >
          {command}
        </span>
        <span className="flex min-w-0 flex-wrap items-center gap-x-1 text-transcript-caption text-muted">
          <span className="shrink-0">{processKindLabel(process.kind)}</span>
          <span aria-hidden="true">·</span>
          <span className="shrink-0">
            started <Time iso={process.started_at} />
          </span>
          {processLinkedByLineage(process) ? (
            <>
              <span aria-hidden="true">·</span>
              <span className="shrink-0 text-faint" title={PROCESS_LINEAGE_HINT}>
                {PROCESS_LINEAGE_LABEL}
              </span>
            </>
          ) : null}
        </span>
        <ProcessPorts ports={process.ports} />
      </span>
      {onStop ? (
        <Button
          type="button"
          variant="ghost"
          size="icon-xs"
          aria-label={`Stop process: ${command}`}
          disabled={stopping || locked}
          onClick={() => onStop(process)}
          className="text-muted"
        >
          {stopping ? (
            <StateGlyph size="sm" state="running" />
          ) : (
            <CircleStop aria-hidden="true" className="size-3.5" />
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
  processes,
  supported = true,
  loading = false,
  failed = false,
  missing = false,
  onRetry,
  onStop,
  stopping = null,
  placeholderCount = 1,
  emptyLabel = "No processes are running.",
}: SessionProcessListProps) {
  // A gone session outranks rows cached from an earlier read.
  if (missing) return <ProcessListNote message="This session is no longer available." />;
  if (!supported) {
    return <ProcessListNote message="Processes can't be listed on this system." />;
  }
  if (processes.length > 0) {
    return (
      <div className="flex flex-col gap-1">
        <ul className="flex max-h-80 flex-col gap-0.5 overflow-y-auto">
          {processes.map(process => {
            const identity = processIdentity(process);
            return (
              <ProcessRow
                key={identity}
                process={process}
                stopping={stopping !== null && stopping !== "all" && stopping.has(identity)}
                locked={stopping === "all"}
                onStop={onStop}
              />
            );
          })}
        </ul>
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
