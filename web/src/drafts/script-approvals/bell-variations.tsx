/**
 * The menubar bell with scripts that wait for approval. Three ways to approve from the bell:
 * A inline on the row, B a peek at the script first, C a section of their own.
 */
import {
  Bell,
  Check,
  ChevronDown,
  ChevronRight,
  FileCode,
  MessageCircleQuestionMark,
  SquareTerminal,
  type LucideIcon,
} from "@compozy/ui/icons";
import { useState, type ReactNode } from "react";

import { Button, cn, Eyebrow, Icon, Pill, Time } from "@compozy/ui";

import { shortHash } from "@/systems/agent/lib/agent-check-in-sources";
import {
  scriptApprovalNoticesFixture,
  scriptPeekFixture,
  type ScriptApprovalNotice,
} from "@/systems/agent/mocks/script-approval-fixtures";

export type BellVariant = "inline" | "peek" | "section";

/** The bell row grid: mark, two lines, trailing slot (same frame as the production row). */
const ROW = "grid w-full grid-cols-[--spacing(4.5)_minmax(0,1fr)_auto] gap-2.5 rounded-md px-2.5";

function Mark({ icon }: { icon: LucideIcon }) {
  return (
    <span
      aria-hidden="true"
      className="mt-0.5 grid size-4.5 shrink-0 place-items-center rounded-full bg-brand-tint text-brand"
    >
      <Icon as={icon} size="xs" />
    </span>
  );
}

/** An ordinary question row, so the new rows read beside what the bell already shows. */
function QuestionRow() {
  return (
    <div className="flex min-w-0 items-center">
      <div className={cn(ROW, "items-center py-1.5")}>
        <Mark icon={MessageCircleQuestionMark} />
        <span className="flex min-w-0 flex-col gap-px">
          <span className="truncate text-small-body font-medium text-fg-strong">
            Fix flaky worktree test
          </span>
          <span className="truncate text-eyebrow text-subtle">
            Code Engineer — asked a question
          </span>
        </span>
        <span className="text-eyebrow text-subtle tabular-nums">
          <Time iso="2026-11-04T15:02:00Z" />
        </span>
      </div>
      <Button aria-label="Mark as read" size="icon-sm" type="button" variant="quiet">
        <Check aria-hidden="true" />
      </Button>
    </div>
  );
}

function reason(notice: ScriptApprovalNotice) {
  return notice.changed ? "changed since you approved" : "waiting for your approval";
}

function Hash({ notice }: { notice: ScriptApprovalNotice }) {
  return (
    <span className="font-mono text-eyebrow text-faint">{shortHash(notice.script.sha256)}</span>
  );
}

/** A: the row says whose script it is, shows the hash, and approves in one click. */
function InlineRow({ notice }: { notice: ScriptApprovalNotice }) {
  return (
    <div className="grid w-full grid-cols-[--spacing(4.5)_minmax(0,1fr)] gap-2.5 rounded-md px-2.5 py-2">
      <Mark icon={SquareTerminal} />
      <span className="flex min-w-0 flex-col gap-0.5">
        <span className="flex min-w-0 items-baseline justify-between gap-2">
          <span className="truncate text-small-body font-medium text-fg-strong">
            {notice.script.name}
          </span>
          <span className="shrink-0 text-eyebrow text-subtle tabular-nums">
            <Time iso={notice.since} />
          </span>
        </span>
        <span className="truncate text-eyebrow text-subtle">
          {notice.agentLabel} — {reason(notice)}
        </span>
        <span className="mt-1 flex items-center gap-1.5">
          <Hash notice={notice} />
          {notice.paused ? (
            <Pill form="plain" tone="neutral">
              Paused
            </Pill>
          ) : null}
          <span className="ml-auto flex items-center gap-1">
            <Button size="xs" type="button" variant="quiet">
              Review
            </Button>
            <Button size="xs" type="button" variant="neutral">
              Approve
            </Button>
          </span>
        </span>
      </span>
    </div>
  );
}

/** B: the row opens a peek at the script before anyone approves it. */
function PeekRow({ notice, defaultOpen }: { notice: ScriptApprovalNotice; defaultOpen: boolean }) {
  const [expanded, setExpanded] = useState(defaultOpen);
  return (
    <div className={cn("flex flex-col rounded-md", expanded && "bg-surface-2")}>
      <button
        aria-expanded={expanded}
        className={cn(ROW, "items-center py-1.5 text-left hover:bg-surface-2")}
        onClick={() => setExpanded(value => !value)}
        type="button"
      >
        <Mark icon={SquareTerminal} />
        <span className="flex min-w-0 flex-col gap-px">
          <span className="truncate text-small-body font-medium text-fg-strong">
            {notice.script.name}
          </span>
          <span className="truncate text-eyebrow text-subtle">
            {notice.agentLabel} — {reason(notice)}
          </span>
        </span>
        <Icon as={expanded ? ChevronDown : ChevronRight} className="text-subtle" size="sm" />
      </button>
      {expanded ? (
        <div className="flex flex-col gap-2 px-2.5 pb-2.5 pl-9">
          <pre className="overflow-hidden rounded-md bg-card px-2.5 py-2 font-mono text-micro leading-relaxed text-muted-foreground">
            {scriptPeekFixture}
          </pre>
          <span className="font-mono text-eyebrow text-faint">
            {shortHash(notice.script.sha256)} · {notice.script.line_count} lines
            {notice.paused ? " · agent paused" : ""}
          </span>
          <span className="flex items-center gap-1.5">
            <Button size="sm" type="button" variant="secondary">
              Approve
            </Button>
            <Button size="sm" type="button" variant="quiet">
              <FileCode aria-hidden="true" data-icon="inline-start" />
              Open script
            </Button>
          </span>
        </div>
      ) : null}
    </div>
  );
}

/** C: one compact line per script, grouped above everything else that needs you. */
function SectionRow({ notice }: { notice: ScriptApprovalNotice }) {
  return (
    <div className={cn(ROW, "items-center py-1.5")}>
      <Mark icon={SquareTerminal} />
      <span className="flex min-w-0 flex-col gap-px">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-small-body font-medium text-fg-strong">
            {notice.script.name}
          </span>
          <Hash notice={notice} />
        </span>
        <span className="truncate text-eyebrow text-subtle">
          {notice.agentLabel}
          {notice.changed ? " · changed" : ""}
          {notice.paused ? " · paused" : ""}
        </span>
      </span>
      <Button size="xs" type="button" variant="neutral">
        Approve
      </Button>
    </div>
  );
}

function BellGroup({
  label,
  count,
  children,
  footer,
}: {
  label: string;
  count: number;
  children: ReactNode;
  footer?: ReactNode;
}) {
  return (
    <section className="flex flex-col border-line pt-1.5 first:pt-0 [&+&]:mt-1 [&+&]:border-t">
      <Eyebrow className="flex items-center gap-1.5 px-2.5 pt-1.5 pb-1 text-subtle">
        {label}
        <span className="font-mono text-micro text-faint">{count}</span>
      </Eyebrow>
      {children}
      {footer}
    </section>
  );
}

/** The bell button, its count, and the open popover as the menubar shows it. */
export function ScriptApprovalBell({ variant }: { variant: BellVariant }) {
  const notices = scriptApprovalNoticesFixture;
  const total = notices.length + 1;
  return (
    <div className="flex justify-end p-6">
      <div className="flex flex-col items-end gap-2">
        <span className="relative">
          <Button aria-label="Notifications" size="icon-sm" type="button" variant="quiet">
            <Bell aria-hidden="true" />
          </Button>
          <span className="absolute -top-0.5 -right-0.5 grid min-w-3.5 place-items-center rounded-full bg-brand px-1 font-mono text-micro text-brand-foreground">
            {total}
          </span>
        </span>
        <div className="flex w-80 flex-col rounded-lg bg-popover p-2 text-small-body text-fg shadow-pop">
          <div className="flex items-center justify-end border-b border-line px-1 pb-2">
            <Button size="sm" type="button" variant="ghost">
              Clear all
            </Button>
          </div>
          <div className="-mx-1 flex flex-col pt-1.5">
            {variant === "section" ? (
              <>
                <BellGroup
                  count={notices.length}
                  footer={
                    <Button className="mx-1 mt-1 self-start" size="xs" type="button" variant="link">
                      Review in Heartbeat
                    </Button>
                  }
                  label="Scripts to approve"
                >
                  {notices.map(notice => (
                    <SectionRow key={notice.id} notice={notice} />
                  ))}
                </BellGroup>
                <BellGroup count={1} label="Needs you">
                  <QuestionRow />
                </BellGroup>
              </>
            ) : (
              <BellGroup count={total} label="Needs you">
                {notices.map((notice, index) =>
                  variant === "peek" ? (
                    <PeekRow key={notice.id} notice={notice} defaultOpen={index === 0} />
                  ) : (
                    <InlineRow key={notice.id} notice={notice} />
                  )
                )}
                <QuestionRow />
              </BellGroup>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
