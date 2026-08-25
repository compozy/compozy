import {
  BellOff,
  CircleHelp,
  CircleSlash,
  Clock3,
  FileX,
  GitBranch,
  ListChecks,
  TriangleAlert,
  type LucideIcon,
} from "lucide-react";

import { Icon, Time } from "@compozy/ui";

import { cn } from "@/lib/utils";
import { SessionBadgeGlyph } from "@/systems/session";

import { LOOP_STORY_ICONS } from "@/systems/loops";
import type { CallAttentionCause } from "@/systems/agent-comms";

import type { OsAttentionRow } from "../lib/attention-model";

function loopIcon(state: "waiting" | "attention"): LucideIcon {
  return state === "waiting" ? LOOP_STORY_ICONS.waiting : LOOP_STORY_ICONS.attention;
}

function NonSessionMark({ icon, tone }: { icon: LucideIcon; tone: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn("grid size-4.5 shrink-0 place-items-center rounded-full", tone)}
    >
      <Icon as={icon} size="xs" />
    </span>
  );
}

/**
 * One red, many icons.
 *
 * All three delegation causes share the needs-you tone and differ by glyph, so
 * colour is never the only channel — the same rule the session badges follow. A
 * coalesced tree gets the branch glyph because the row is about the tree, not
 * about any one call in it.
 */
const CALL_CAUSE_ICON: Record<CallAttentionCause, LucideIcon> = {
  "invalid-result": FileX,
  "completed-without-result": CircleSlash,
  "blocked-on-decision": CircleHelp,
};

function RowMark({ row }: { row: OsAttentionRow }) {
  if (row.kind === "session") return <SessionBadgeGlyph badge={row.badge} />;
  if (row.kind === "task") {
    return <NonSessionMark icon={ListChecks} tone="bg-danger-tint text-danger" />;
  }
  if (row.kind === "call") {
    return (
      <NonSessionMark
        icon={row.count > 1 ? GitBranch : CALL_CAUSE_ICON[row.cause]}
        tone="bg-danger-tint text-danger"
      />
    );
  }
  if (row.kind === "loop-request") {
    return <NonSessionMark icon={TriangleAlert} tone="bg-danger-tint text-danger" />;
  }
  return (
    <NonSessionMark
      icon={loopIcon(row.state)}
      tone={row.state === "waiting" ? "bg-info-tint text-info" : "bg-warning-tint text-warning"}
    />
  );
}

function rowReason(row: OsAttentionRow): string {
  if (row.kind === "session") return `${row.agentName} — ${row.reason}`;
  if (row.kind === "task") return "task approval";
  if (row.kind === "call") return row.rootSessionId;
  if (row.kind === "loop-request") return `${row.loopName} — ${row.requestKind}`;
  return row.state;
}

export interface AttentionBellRowProps {
  row: OsAttentionRow;
  onSelect: (row: OsAttentionRow) => void;
}

/**
 * One attention row: mark, title, why it is here, which workspace it belongs
 * to, and how long it has waited. A muted workspace still shows its row and
 * still counts — the mute mark makes the silence visible rather than
 * mysterious. A stale row dims but stays activatable as a fallback jump.
 */
export function AttentionBellRow({ row, onSelect }: AttentionBellRowProps) {
  const isSession = row.kind === "session";
  const isLoopRequest = row.kind === "loop-request";
  const isCall = row.kind === "call";
  const stale = (isSession || isLoopRequest || isCall) && row.stale;
  const muted = isSession && row.muted;
  return (
    <button
      type="button"
      className={cn(
        "grid w-full grid-cols-[--spacing(4.5)_minmax(0,1fr)_auto] items-center gap-2.5 rounded-md px-2.5 py-1.5 text-left transition-colors hover:bg-row-hover focus-visible:bg-row-hover focus-visible:shadow-focus-ring focus-visible:outline-none",
        stale && "opacity-60"
      )}
      data-testid={`os-attention-${row.kind}-${row.id}`}
      data-muted={muted ? "true" : undefined}
      data-stale={stale ? "true" : undefined}
      onClick={() => onSelect(row)}
    >
      <RowMark row={row} />
      <span className="flex min-w-0 flex-col gap-px">
        <span className="truncate text-small-body font-medium text-fg-strong">{row.title}</span>
        <span className="flex min-w-0 items-center gap-1.5 text-micro text-subtle">
          <span className="truncate">{rowReason(row)}</span>
          {isSession || isLoopRequest ? (
            <span className="shrink-0 font-mono text-micro text-faint">{row.workspaceLabel}</span>
          ) : null}
        </span>
      </span>
      <span className="flex shrink-0 items-center gap-1 font-mono text-micro text-subtle">
        {muted ? (
          <Icon as={BellOff} size="xs" className="text-faint" aria-label="Notifications muted" />
        ) : null}
        {isSession || isCall ? <Time iso={row.changedAt} /> : null}
        {isLoopRequest ? (
          <span className="flex flex-col items-end">
            <Time iso={row.openedAt} />
            {row.expiresAt ? (
              <span className="flex items-center gap-0.5 text-danger">
                <Icon as={Clock3} size="xs" />
                <Time iso={row.expiresAt} />
              </span>
            ) : null}
          </span>
        ) : null}
      </span>
    </button>
  );
}
