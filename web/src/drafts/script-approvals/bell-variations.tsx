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

import { Button, CodeBlock, cn, Eyebrow, Icon, Time } from "@compozy/ui";

import { shortHash } from "@/systems/agent/lib/agent-check-in-sources";
import {
  scriptApprovalNoticesFixture,
  scriptPeekFixture,
  type ScriptApprovalNotice,
} from "@/systems/agent/mocks/script-approval-fixtures";

import { SCRIPT_BELL_TOTAL } from "./fleet-fixtures";

export type BellVariant = "inline" | "peek" | "section";

/** The bell row grid: mark, two lines, trailing slot (same frame as the production row). */
const ROW = "grid w-full grid-cols-[--spacing(4.5)_minmax(0,1fr)_auto] gap-2.5 rounded-md px-2.5";

function Mark({ icon }: { icon: LucideIcon }) {
  return (
    <span
      aria-hidden="true"
      className="grid size-4.5 shrink-0 place-items-center rounded-full bg-attn-tint text-attn"
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

/** Why it's in the bell, in the agent's words; the hash stays in the script. */
function reason(notice: ScriptApprovalNotice) {
  return notice.changed ? "script changed" : "new script";
}

function Hash({ notice }: { notice: ScriptApprovalNotice }) {
  return (
    <span className="font-mono text-eyebrow text-faint">{shortHash(notice.script.sha256)}</span>
  );
}

/** The production row's button: the whole row opens the script on Heartbeat. */
const ROW_BUTTON = cn(
  ROW,
  "items-center py-2 text-left transition-colors hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
);

/** The production row's two lines: what, then who and why. */
function RowLines({ notice }: { notice: ScriptApprovalNotice }) {
  return (
    <span className="flex min-w-0 flex-col gap-px">
      <span className="truncate text-small-body font-medium text-fg-strong">
        {notice.script.name}
      </span>
      <span className="truncate text-eyebrow text-subtle">
        {notice.agentLabel} — {reason(notice)}
      </span>
    </span>
  );
}

/** A: the bell's own row, with Approve in the trailing slot so the reason reads in full. */
function InlineRow({ notice }: { notice: ScriptApprovalNotice }) {
  return (
    <div className="flex min-w-0 items-center gap-1 pr-1">
      <button
        aria-label={`Review ${notice.script.name} on Heartbeat`}
        className={ROW_BUTTON}
        type="button"
      >
        <Mark icon={SquareTerminal} />
        <RowLines notice={notice} />
        <span />
      </button>
      <Button size="xs" type="button" variant="secondary">
        Approve
      </Button>
    </div>
  );
}

/** B: the same row opens onto the first lines of the script, then asks. */
function PeekRow({ notice, defaultOpen }: { notice: ScriptApprovalNotice; defaultOpen: boolean }) {
  const [expanded, setExpanded] = useState(defaultOpen);
  return (
    <div className={cn("flex flex-col rounded-md", expanded && "bg-surface-2")}>
      <button
        aria-expanded={expanded}
        className={ROW_BUTTON}
        onClick={() => setExpanded(value => !value)}
        type="button"
      >
        <Mark icon={SquareTerminal} />
        <RowLines notice={notice} />
        <Icon as={expanded ? ChevronDown : ChevronRight} className="text-subtle" size="sm" />
      </button>
      {expanded ? (
        <div className="flex flex-col gap-3 px-2.5 pb-3">
          <CodeBlock
            code={scriptPeekFixture}
            copyable={false}
            density="compact"
            language="bash"
            truncateLines={5}
            wrapLines
          />
          <div className="flex items-center justify-between gap-2">
            <span className="text-eyebrow text-subtle tabular-nums">
              {notice.script.line_count} lines · changed <Time iso={notice.since} />
            </span>
            <span className="flex items-center gap-1">
              <Button size="xs" type="button" variant="ghost">
                <FileCode aria-hidden="true" data-icon="inline-start" />
                Open
              </Button>
              <Button size="xs" type="button" variant="secondary">
                Approve
              </Button>
            </span>
          </div>
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

/** What the open bell shows, inside the menubar's popover or a stand-in for it. */
export function ScriptApprovalBellPanel({ variant }: { variant: BellVariant }) {
  const notices = scriptApprovalNoticesFixture;
  const total = SCRIPT_BELL_TOTAL;
  return (
    <div className="flex min-h-0 flex-col text-small-body text-fg">
      <div className="flex items-center justify-end border-b border-line px-1 pb-2">
        <Button size="sm" type="button" variant="ghost">
          Clear all
        </Button>
      </div>
      <div className="-mx-1 flex max-h-[min(36rem,70vh)] flex-col overflow-y-auto pt-1.5">
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
  );
}

/** The bell button, its count, and the open popover as the menubar shows it. */
export function ScriptApprovalBell({ variant }: { variant: BellVariant }) {
  return (
    <div className="flex justify-end p-6">
      <div className="flex flex-col items-end gap-2">
        <span className="relative">
          <Button aria-label="Notifications" size="icon-sm" type="button" variant="quiet">
            <Bell aria-hidden="true" />
          </Button>
          <span className="absolute -top-0.5 -right-0.5 grid min-w-3.5 place-items-center rounded-full bg-attn px-1 font-mono text-micro text-canvas">
            {SCRIPT_BELL_TOTAL}
          </span>
        </span>
        <div className="w-80 rounded-lg bg-popover p-2 shadow-pop">
          <ScriptApprovalBellPanel variant={variant} />
        </div>
      </div>
    </div>
  );
}
