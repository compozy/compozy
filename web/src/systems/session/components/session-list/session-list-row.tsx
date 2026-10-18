import { useRef, useState } from "react";

import { Checkbox, Time } from "@compozy/ui";
import type { SessionRowSelection } from "../../hooks/use-session-selection";

import { cn } from "@/lib/utils";

import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { sessionBadgeSignal } from "../../lib/session-badge";
import { sessionBadgeWordClass } from "../../lib/session-badge-classes";
import { sessionAwaitsSubagents, sessionSubagentCounts } from "../../lib/session-subagent-summary";
import { maskedAttentionNote } from "../../lib/session-pending-interactions";
import {
  sessionAttentionReason,
  type SessionAttentionReason,
} from "../../lib/session-attention-reason";
import { useSessionRuntimeLabel } from "../../hooks/use-session-runtime-label";
import { SessionBadgeMark } from "../session-badge-mark";
import type { SessionPayload } from "../../types";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { SessionRowActions } from "../session-row-actions";
import { SessionRowContextMenu, type SessionRowLayoutMenu } from "../session-row-context-menu";
import type { SessionDragHandlers, SessionDropPosition } from "../../hooks/use-session-list-dnd";
import { SUBAGENT_OF_DELETED_SESSION } from "../subagents/subagent-format";
import { SessionListProviderMark } from "./session-list-provider-mark";
import { SessionListSubagentChip } from "./session-list-subagent-chip";
import { ProfileOwnerTag, type ProfileOwner } from "@/systems/profiles";

export interface SessionListRowProps {
  session: SessionPayload;
  /**
   * The row's profile, supplied only while the aggregate is on and only where it
   * adds information: a scoped list already answers "whose work is this", and a
   * child in its root's profile repeats it (US-011.AC-1).
   */
  owner?: ProfileOwner;
  /**
   * The thread root of a child row. A child running the same agent on the same
   * provider inherits the root's meta line and renders as a single line.
   */
  parent?: SessionPayload;
  current?: boolean;
  onSelect: () => void;
  sessionActions: SessionLifecycleActionHandlers;
  showActions?: boolean;
  testIdPrefix: string;
  /** A subagent session whose ancestry was deleted: it leads its thread, labelled. */
  orphanedSubagent?: boolean;
  /** Controls rendered in the meta line's trailing cluster (e.g. a thread toggle). */
  trailing?: React.ReactNode;
  selection?: SessionRowSelection;
  /** Folder moves and manual-order verbs for a root row in the workspace list. */
  layoutMenu?: SessionRowLayoutMenu;
  /** Drag source/target wiring for a root row in the workspace list. */
  drag?: SessionDragHandlers;
  /** The pending drop lands on this edge of the row. */
  dropEdge?: SessionDropPosition | null;
  /** This row is the one being dragged. */
  dragging?: boolean;
}

/**
 * Two-line session row. Line 1 belongs to the title: only the end slot shares it,
 * holding the compact time at rest and the row actions on hover or focus. Line 2
 * carries who runs it (owner, provider, agent), attention words, and the trailing
 * cluster. The title button's `::after` stretches over the row so the whole row
 * navigates, while the checkbox, chip, toggle, and actions sit above it as
 * siblings — no interactive control nests inside another.
 */
export function SessionListRow(props: SessionListRowProps) {
  const { session, sessionActions, showActions = true, layoutMenu } = props;
  const row = (
    <SessionRowDragFrame
      drag={props.drag}
      dropEdge={props.dropEdge ?? null}
      dragging={props.dragging ?? false}
      dropTestId={`${props.testIdPrefix}-drop-${props.dropEdge}-${session.id}`}
    >
      <SessionListRowBody {...props} />
    </SessionRowDragFrame>
  );
  if (!showActions) return row;
  return (
    <SessionRowContextMenu
      session={session}
      actions={sessionActions}
      layout={layoutMenu}
      render={row}
    />
  );
}

function SessionListRowBody({
  session,
  owner,
  parent,
  current = false,
  onSelect,
  sessionActions,
  showActions = true,
  testIdPrefix,
  trailing,
  selection,
  orphanedSubagent = false,
}: SessionListRowProps) {
  const selected = selection?.selectedIds.has(session.id) ?? false;
  const title = getSessionDisplayTitle(session);
  const subagentCounts = sessionSubagentCounts(session);
  const runtime = useSessionRuntimeLabel(session.runtime?.effective);
  const signal = sessionBadgeSignal(session.badge);
  // Each keyboard focus gets its own id; pointer focus reveals nothing.
  const focusCount = useRef(0);
  const [keyboardFocus, setKeyboardFocus] = useState<number | null>(null);
  const actionsVisible = showActions && !selection?.mode;
  const meta = rowMeta({ session, parent, owner, runtime, signal, orphanedSubagent });
  const hasTrailing = subagentCounts !== null || trailing !== undefined;
  return (
    <div
      className={cn(
        "group/session-row relative grid grid-cols-[12px_minmax(0,1fr)_auto] items-center gap-x-2 gap-y-0.75 rounded-md px-2 py-1.5 transition-colors hover:bg-surface-2 has-[[data-slot=session-row-target]:focus-visible]:shadow-focus-ring",
        (selected || current) && "bg-selected"
      )}
      data-slot="session-list-row"
    >
      <button
        type="button"
        className={cn(
          "col-span-2 col-start-1 row-start-1 grid min-w-0 grid-cols-[12px_minmax(0,1fr)] items-center gap-x-2 text-left text-body text-fg outline-none after:absolute after:inset-0 after:rounded-md",
          (signal.attention === "needs-you" || current) && "font-medium"
        )}
        data-slot="session-row-target"
        // The meta line sits under the target, so the attention detail rides on the row.
        title={meta.reason?.detail}
        data-status={session.badge}
        data-testid={`${testIdPrefix}-session-${session.id}`}
        aria-current={current ? "true" : undefined}
        // While a selection is open, activating the row toggles its membership,
        // so the row reads as a toggle button; otherwise it only navigates.
        aria-pressed={selection?.mode ? selected : undefined}
        data-selected={selected || undefined}
        onClick={event => activateSessionRow(event, session.id, selection, onSelect)}
        onFocus={event => {
          focusCount.current += 1;
          setKeyboardFocus(
            event.currentTarget.matches(":focus-visible") ? focusCount.current : null
          );
        }}
        onBlur={() => setKeyboardFocus(null)}
      >
        <SessionBadgeMark
          badge={session.badge}
          delegated={sessionAwaitsSubagents(session)}
          className={cn(
            selection &&
              "transition-opacity duration-fast motion-reduce:transition-none group-hover/session-row:opacity-0 group-focus-within/session-row:opacity-0",
            selection?.mode && "opacity-0"
          )}
        />
        <span className="min-w-0 truncate">{title}</span>
        {/* The meta line is drawn outside the target; its words still name the row. */}
        <span className="sr-only">{meta.spoken}</span>
      </button>
      {selection ? (
        <SessionRowCheckbox
          title={title}
          sessionId={session.id}
          selected={selected}
          selection={selection}
        />
      ) : null}
      <span className="col-start-3 row-start-1 grid min-w-6 items-center justify-items-end *:col-start-1 *:row-start-1">
        <Time
          iso={session.updated_at}
          mode="compact"
          className={cn(
            "font-mono text-micro text-subtle transition-opacity duration-fast motion-reduce:transition-none",
            actionsVisible &&
              "group-focus-within/session-row:opacity-0 group-hover/session-row:opacity-0 group-has-data-popup-open/session-row:opacity-0"
          )}
        />
        {actionsVisible ? (
          <span className="relative z-10 -my-1 opacity-0 transition-opacity duration-fast group-focus-within/session-row:opacity-100 group-hover/session-row:opacity-100 group-has-data-popup-open/session-row:opacity-100 motion-reduce:transition-none">
            <SessionRowActions session={session} actions={sessionActions} />
          </span>
        ) : null}
      </span>
      {meta.visible || hasTrailing ? (
        <span className="col-span-2 col-start-2 row-start-2 flex min-w-0 items-center gap-2">
          <span className="flex min-w-0 flex-1 items-center gap-1.25 overflow-hidden text-eyebrow whitespace-nowrap text-subtle">
            {owner ? <ProfileOwnerTag compact size="xs" owner={owner} /> : null}
            {meta.inherits ? null : (
              <SessionListProviderMark
                sessionId={session.id}
                runtime={runtime}
                keyboardFocus={keyboardFocus}
              />
            )}
            <SessionListRowWho session={session} meta={meta} />
          </span>
          {hasTrailing ? (
            <span className="relative z-10 -mr-0.5 flex shrink-0 items-center gap-0.5">
              {subagentCounts !== null ? (
                <SessionListSubagentChip
                  session={session}
                  counts={subagentCounts}
                  onSelect={onSelect}
                />
              ) : null}
              {trailing}
            </span>
          ) : null}
        </span>
      ) : null}
    </div>
  );
}

function activateSessionRow(
  event: React.MouseEvent<HTMLButtonElement>,
  sessionId: string,
  selection: SessionRowSelection | undefined,
  onSelect: () => void
) {
  if (selection && event.shiftKey) selection.toggleRange(sessionId);
  else if (selection && (selection.mode || event.metaKey || event.ctrlKey))
    selection.toggle(sessionId);
  else onSelect();
}

function SessionRowCheckbox({
  title,
  sessionId,
  selected,
  selection,
}: {
  title: string;
  sessionId: string;
  selected: boolean;
  selection: SessionRowSelection;
}) {
  return (
    <Checkbox
      aria-label={`Select ${title}`}
      checked={selected}
      className={cn(
        "absolute top-2 left-1 z-10 transition-opacity duration-fast motion-reduce:transition-none after:inset-0",
        selection.mode
          ? "opacity-100"
          : "pointer-events-none opacity-0 group-hover/session-row:pointer-events-auto group-hover/session-row:opacity-100 group-focus-within/session-row:pointer-events-auto group-focus-within/session-row:opacity-100"
      )}
      onCheckedChange={(_checked, details) => {
        if ("shiftKey" in details.event && details.event.shiftKey) selection.toggleRange(sessionId);
        else selection.toggle(sessionId);
      }}
    />
  );
}

/** The drag source/target frame; it draws the drop line on the pending edge. */
function SessionRowDragFrame({
  drag,
  dropEdge,
  dragging,
  dropTestId,
  children,
  className,
  ...rest
}: React.ComponentProps<"div"> & {
  drag: SessionDragHandlers | undefined;
  dropEdge: SessionDropPosition | null;
  dragging: boolean;
  dropTestId: string;
}) {
  return (
    <div
      {...rest}
      {...drag}
      data-dragging={dragging || undefined}
      className={cn(
        "relative",
        dragging && "opacity-50",
        className
      )}
    >
      {dropEdge !== null ? <SessionRowDropLine edge={dropEdge} testId={dropTestId} /> : null}
      {children}
    </div>
  );
}

/** The drop indicator on the edge where a dragged session will land. */
function SessionRowDropLine({ edge, testId }: { edge: SessionDropPosition; testId: string }) {
  return (
    <span
      aria-hidden="true"
      data-testid={testId}
      className={cn(
        "pointer-events-none absolute inset-x-1 z-10 h-0.5 rounded-full bg-accent",
        edge === "before" ? "-top-px" : "-bottom-px"
      )}
    />
  );
}

interface RowMeta {
  /** Same agent and provider as the thread root: the child drops its who line. */
  inherits: boolean;
  /** The meta line has something of its own to show. */
  visible: boolean;
  /** Attention words that never truncate. */
  attention: string[];
  /** Why the daemon raised attention; its detail is the row's hover title. */
  reason: SessionAttentionReason | null;
  /** Secondary notes, the first to give way when the line is full. */
  notes: string[];
  /** The row's accessible description, in reading order. */
  spoken: string;
}

function rowMeta({
  session,
  parent,
  owner,
  runtime,
  signal,
  orphanedSubagent,
}: {
  session: SessionPayload;
  parent: SessionPayload | undefined;
  owner: ProfileOwner | undefined;
  runtime: { label: string } | null;
  signal: ReturnType<typeof sessionBadgeSignal>;
  orphanedSubagent: boolean;
}): RowMeta {
  const inherits =
    parent !== undefined &&
    owner === undefined &&
    parent.agent_name === session.agent_name &&
    (parent.runtime?.effective?.provider ?? null) ===
      (session.runtime?.effective?.provider ?? null);
  const attention: string[] = [];
  // The mark carries the state; the word speaks up whenever the glyph is shared.
  if (signal.speaks) attention.push(signal.displayLabel);
  const reason = sessionAttentionReason(session, signal.label);
  if (reason !== null) attention.push(reason.summary);
  const masked = maskedAttentionNote(session, signal.label);
  if (masked !== null) attention.push(masked);
  const notes: string[] = [];
  if (orphanedSubagent) notes.push(SUBAGENT_OF_DELETED_SESSION);
  if (session.archived_at !== null) notes.push("Archived");
  const spoken = [runtime?.label, [session.agent_name, ...attention, ...notes].join(" · ")]
    .filter(Boolean)
    .join(" ");
  return {
    inherits,
    visible: !inherits || attention.length > 0 || notes.length > 0,
    attention,
    reason,
    notes,
    spoken,
  };
}

/**
 * The visible who line. Truncation order when the line is full: notes give way
 * first, then the agent name, and attention words only as a last resort. Each
 * separator lives inside the neighbour that gives way first, so a collapsed part
 * never leaves a stray `·`. The words repeat the target's accessible name, so the
 * line itself stays out of the a11y tree.
 */
function SessionListRowWho({ session, meta }: { session: SessionPayload; meta: RowMeta }) {
  const parts: { key: string; kind: "agent" | "attention" | "note"; text: string }[] = [];
  if (!meta.inherits) parts.push({ key: "agent", kind: "agent", text: session.agent_name });
  meta.attention.forEach((text, index) =>
    parts.push({ key: `attention-${index}`, kind: "attention", text })
  );
  meta.notes.forEach((text, index) => parts.push({ key: `note-${index}`, kind: "note", text }));
  return (
    <>
      {parts.map((part, index) => {
        const next = parts[index + 1];
        const leading = index > 0 && part.kind === "note";
        const trailing = next !== undefined && next.kind !== "note";
        return (
          <span
            key={part.key}
            aria-hidden="true"
            className={cn(
              "min-w-0 truncate",
              part.kind === "agent" && "shrink-[100] font-medium text-muted",
              part.kind === "attention" && "shrink",
              part.kind === "attention" &&
                index === firstAttention(parts) &&
                sessionBadgeWordClass(session.badge),
              part.kind === "note" && "shrink-[10000]"
            )}
          >
            {leading ? <span className="text-faint">· </span> : null}
            {part.text}
            {trailing ? <span className="text-faint"> ·</span> : null}
          </span>
        );
      })}
    </>
  );
}

function firstAttention(parts: { kind: string }[]): number {
  return parts.findIndex(part => part.kind === "attention");
}
