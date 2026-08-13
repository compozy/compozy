import { CircleAlertIcon, GitMergeIcon } from "lucide-react";
import type { ReactNode } from "react";

import { Eyebrow, MonoId, Pill, cn } from "@compozy/ui";

import type { WorktreeForgeCapabilities, WorktreeStatusPayload } from "../types";
import { WorktreeAheadBehindSignal, WorktreeDirtySignal } from "./worktree-signal-parts";

interface WorktreeStatusStripProps {
  status: WorktreeStatusPayload["status"];
  forgeStatus?: WorktreeStatusPayload["forge"] | undefined;
  /** Absent ⇒ no forge serves this remote: the PR cell does not exist. */
  forge?: WorktreeForgeCapabilities;
  /** Relative stamps for remote-derived values, when they have aged. */
  staleLabel?: string;
}

interface CellProps {
  field: string;
  label: string;
  children: ReactNode;
  tone?: "success" | "warning" | "danger" | "neutral";
  unknown?: boolean;
  detached?: boolean;
}

function Cell({ field, label, children, tone, unknown, detached }: CellProps) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-[3px] px-3.5 py-2.5",
        "[&+&]:border-l [&+&]:border-line-soft",
        "max-md:[&+&]:border-t max-md:[&+&]:border-l-0"
      )}
      data-detached={detached ? "" : undefined}
      data-field={field}
      data-slot="worktree-status-cell"
      data-tone={tone}
      data-unknown={unknown ? "" : undefined}
    >
      <dt>
        <Eyebrow>{label}</Eyebrow>
      </dt>
      <dd
        aria-label={unknown ? "not read" : undefined}
        className={cn(
          "flex min-h-[18px] items-center gap-1.5 text-small-body font-medium text-fg",
          tone === "danger" && "text-danger",
          tone === "warning" && "text-warning",
          tone === "success" && "text-success",
          unknown && "font-normal text-faint"
        )}
        data-slot="worktree-status-value"
      >
        {children}
      </dd>
    </div>
  );
}

function BranchCell({ status }: { status: WorktreeStatusPayload["status"] }) {
  if (status.detached === true && status.head_sha) {
    return (
      <Cell detached field="branch" label="Branch">
        <span className="text-muted">pinned at </span>
        <MonoId preserveCase size="sm" value={status.head_sha.slice(0, 7)} />
      </Cell>
    );
  }
  if (!status.branch) {
    return (
      <Cell field="branch" label="Branch" unknown>
        —
      </Cell>
    );
  }
  return (
    <Cell field="branch" label="Branch">
      <MonoId copy copyLabel="Copy branch" preserveCase size="sm" value={status.branch} />
    </Cell>
  );
}

function DirtyCell({ status }: { status: WorktreeStatusPayload["status"] }) {
  if (status.dirty_files == null) {
    return (
      <Cell field="dirty" label="Dirty" unknown>
        —
      </Cell>
    );
  }
  if (status.dirty_files === 0) {
    return (
      <Cell field="dirty" label="Dirty" tone="neutral">
        <span className="font-normal text-muted">clean</span>
      </Cell>
    );
  }
  const hasCounts = status.insertions != null || status.deletions != null;
  return (
    <Cell field="dirty" label="Dirty" tone="warning">
      {hasCounts ? (
        <WorktreeDirtySignal deletions={status.deletions} dirty insertions={status.insertions} />
      ) : (
        <span>uncommitted</span>
      )}
      <span className="text-micro text-subtle">{`${status.dirty_files} files`}</span>
    </Cell>
  );
}

function PrCell({
  forge,
  forgeStatus,
  staleLabel,
}: {
  forge: WorktreeForgeCapabilities;
  forgeStatus: WorktreeStatusPayload["forge"] | undefined;
  staleLabel?: string;
}) {
  const label = forge.request_noun;
  if (!forgeStatus?.pr_state) {
    return (
      <Cell field="pr" label={label} unknown>
        —
      </Cell>
    );
  }
  const number = forgeStatus.pr_number == null ? "" : `#${forgeStatus.pr_number}`;
  if (forgeStatus.merged === true) {
    return (
      <Cell field="pr" label={label} tone="success">
        <GitMergeIcon aria-hidden="true" className="size-3" />
        <span>{`Merged on ${forge.provider}`}</span>
        {number ? <span className="text-micro text-subtle">{number}</span> : null}
        {staleLabel ? <span className="text-micro text-faint">{`as of ${staleLabel}`}</span> : null}
      </Cell>
    );
  }
  return (
    <Cell field="pr" label={label} tone={forgeStatus.pr_state === "closed" ? "neutral" : undefined}>
      {number ? (
        <Pill mono size="xs" tone={forgeStatus.pr_state === "open" ? "info" : "neutral"}>
          {`${label} ${number}`}
        </Pill>
      ) : null}
      <span className="text-muted">{forgeStatus.pr_state}</span>
      {staleLabel ? <span className="text-micro text-faint">{`as of ${staleLabel}`}</span> : null}
    </Cell>
  );
}

/**
 * Five git facts, and only what was actually read.
 *
 * A fact nobody could read renders as an em-dash rather than a zero, ahead and
 * behind appear only against a known upstream, and the whole strip loses its PR
 * cell when no forge serves the remote. The read-failure cell exists only when
 * a read actually failed — and when it does, it is the reason every exit action
 * is blocked.
 */
export function WorktreeStatusStrip({
  status,
  forgeStatus,
  forge,
  staleLabel,
}: WorktreeStatusStripProps) {
  const showAheadBehind = status.has_upstream === true;
  const aheadBehindUnknown = status.ahead == null && status.behind == null;

  return (
    <dl
      className="flex flex-wrap items-center overflow-hidden rounded-lg border border-line bg-canvas-soft max-md:flex-col max-md:items-stretch"
      data-slot="worktree-status-strip"
    >
      <BranchCell status={status} />
      <DirtyCell status={status} />
      {showAheadBehind ? (
        aheadBehindUnknown ? (
          <Cell field="ahead-behind" label="Ahead / behind" unknown>
            —
          </Cell>
        ) : (
          <Cell
            field="ahead-behind"
            label="Ahead / behind"
            tone={(status.behind ?? 0) > 0 ? "warning" : undefined}
          >
            <WorktreeAheadBehindSignal ahead={status.ahead} behind={status.behind} />
            {staleLabel ? (
              <span className="text-micro text-faint">{`as of ${staleLabel}`}</span>
            ) : null}
          </Cell>
        )
      ) : null}
      {forge ? (
        <PrCell forge={forge} forgeStatus={forgeStatus ?? null} staleLabel={staleLabel} />
      ) : null}
      {status.read_error ? (
        <div
          aria-live="polite"
          className="flex min-w-0 flex-col gap-[3px] border-l border-line-soft px-3.5 py-2.5 max-md:border-t max-md:border-l-0"
          data-field="status"
          data-slot="worktree-status-cell"
          data-tone="danger"
          role="status"
        >
          <dt>
            <Eyebrow>Status</Eyebrow>
          </dt>
          <dd
            className="flex min-h-[18px] items-center gap-1.5 text-small-body font-medium text-danger"
            data-slot="worktree-status-value"
          >
            <CircleAlertIcon aria-hidden="true" className="size-3" />
            <span>read failed</span>
            <span className="text-micro text-subtle">{status.read_error}</span>
          </dd>
        </div>
      ) : null}
    </dl>
  );
}
