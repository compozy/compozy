/**
 * One session's calls, both directions, in the inspector rail.
 *
 * Direction is carried by an arrow, never by colour: help this session asked for
 * points out, help it was asked for points in. Colour in this feature means
 * state and nothing else, so spending it on direction would break the one rule
 * the whole signal grammar rests on.
 *
 * **Counts are the daemon's, not the list's.** Each section's chip shows the
 * `total` the daemon computed over the filtered population, which is why a
 * section can honestly read "247" while showing 25 rows. Deriving the number
 * from `rows.length` would quietly turn a page size into a fact — the exact
 * failure the truthful-count rule exists to prevent.
 */
import { ArrowDownLeft, ArrowUpRight } from "lucide-react";

import { Button, Empty, Eyebrow, OwnerAvatar, Time } from "@compozy/ui";

import { AgentCallStatePill, AgentCallVerdictChip } from "./agent-call-state-pill";
import { toCallState, toCallVerdict } from "../lib/call-state";
import type { CallPayload } from "../types";

export interface CallDirectionSection {
  /** Rows loaded so far. */
  calls: readonly CallPayload[];
  /** The daemon's count for this direction. Undefined while the probe is in flight. */
  total: number | undefined;
  hasMore: boolean;
  onLoadMore: () => void;
  loadingMore?: boolean;
}

export interface AgentCallsInspectorPanelProps {
  made: CallDirectionSection;
  received: CallDirectionSection;
  onOpenCall: (callId: string) => void;
  /**
   * Session ids that no longer resolve. A call whose counterpart was pruned by
   * retention keeps its record and its identities; only the jump degrades.
   */
  prunedSessionIds?: ReadonlySet<string>;
  "data-testid"?: string;
}

function CallRow({
  call,
  direction,
  pruned,
  onOpenCall,
}: {
  call: CallPayload;
  direction: "made" | "received";
  pruned: boolean;
  onOpenCall: (callId: string) => void;
}) {
  const Arrow = direction === "made" ? ArrowUpRight : ArrowDownLeft;
  const counterpart =
    direction === "made" ? (call.agent ?? call.child_session_id ?? "") : call.caller.id;
  return (
    <li
      data-testid="agent-calls-panel-row"
      data-call-id={call.call_id}
      data-pruned={pruned || undefined}
      className="flex items-center gap-2 py-1"
    >
      <Arrow className="size-3 shrink-0 text-muted" aria-hidden="true" />
      <OwnerAvatar ownerKind="agent" ownerId={counterpart || call.call_id} size="sm" />
      {pruned ? (
        // Identity stays; the link is absent, and the row says why.
        <span className="min-w-0 flex-1 truncate text-form text-fg">{counterpart}</span>
      ) : (
        <button
          type="button"
          className="min-w-0 flex-1 truncate text-left text-form text-fg hover:underline"
          onClick={() => onOpenCall(call.call_id)}
        >
          {counterpart}
        </button>
      )}
      <AgentCallStatePill state={toCallState(call.state)} fallbackLabel={call.state} />
      <AgentCallVerdictChip verdict={toCallVerdict(call.verdict)} />
      {pruned ? (
        <span className="shrink-0 text-form text-muted">session pruned — record retained</span>
      ) : null}
      <Time iso={call.updated_at} className="shrink-0 text-form text-muted" />
    </li>
  );
}

function Section({
  label,
  icon: Icon,
  section,
  direction,
  prunedSessionIds,
  onOpenCall,
}: {
  label: string;
  icon: typeof ArrowUpRight;
  section: CallDirectionSection;
  direction: "made" | "received";
  prunedSessionIds: ReadonlySet<string>;
  onOpenCall: (callId: string) => void;
}) {
  const loaded = section.calls.length;
  return (
    <section data-testid={`agent-calls-panel-${direction}`} className="flex flex-col gap-1">
      <header className="flex items-center gap-1.5">
        <Icon className="size-3 shrink-0 text-muted" aria-hidden="true" />
        <Eyebrow>{label}</Eyebrow>
        <span className="flex-1" />
        {section.total !== undefined ? (
          <span
            className="font-mono text-form text-muted"
            data-testid={`agent-calls-panel-${direction}-count`}
          >
            {section.total}
          </span>
        ) : null}
      </header>

      {loaded === 0 ? (
        <p className="py-1 text-form text-muted">
          {section.total === 0 ? "None yet." : "Loading…"}
        </p>
      ) : (
        <ul className="flex flex-col divide-y divide-line-soft">
          {section.calls.map(call => {
            const counterpartId =
              direction === "made" ? (call.child_session_id ?? "") : call.caller.id;
            return (
              <CallRow
                key={call.call_id}
                call={call}
                direction={direction}
                pruned={counterpartId !== "" && prunedSessionIds.has(counterpartId)}
                onOpenCall={onOpenCall}
              />
            );
          })}
        </ul>
      )}

      {section.hasMore ? (
        <div className="flex items-center gap-2 pt-1">
          {/*
            The pager states what it knows: how many are loaded out of the real
            total. It never claims more precision than the daemon gave it.
          */}
          <span className="text-form text-muted">
            {section.total === undefined
              ? `${loaded} loaded`
              : `${loaded} of ${section.total} loaded`}
          </span>
          <span className="flex-1" />
          <Button
            size="xs"
            variant="outline"
            type="button"
            disabled={section.loadingMore}
            onClick={section.onLoadMore}
            data-testid={`agent-calls-panel-${direction}-more`}
          >
            Load older
          </Button>
        </div>
      ) : null}
    </section>
  );
}

const NO_PRUNED_SESSIONS: ReadonlySet<string> = new Set();

export function AgentCallsInspectorPanel({
  made,
  received,
  onOpenCall,
  prunedSessionIds = NO_PRUNED_SESSIONS,
  "data-testid": testId,
}: AgentCallsInspectorPanelProps) {
  const empty = made.total === 0 && received.total === 0;
  if (empty) {
    return (
      <Empty
        data-testid={testId}
        title="No calls yet"
        description="When this session delegates work — or is delegated to — the exchange shows up here."
      />
    );
  }
  return (
    <div data-testid={testId} className="flex flex-col gap-4">
      <Section
        label="Made"
        icon={ArrowUpRight}
        section={made}
        direction="made"
        prunedSessionIds={prunedSessionIds}
        onOpenCall={onOpenCall}
      />
      <Section
        label="Received"
        icon={ArrowDownLeft}
        section={received}
        direction="received"
        prunedSessionIds={prunedSessionIds}
        onOpenCall={onOpenCall}
      />
    </div>
  );
}
