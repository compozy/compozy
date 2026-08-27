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
import { ActionResultBanner, Button, Empty, ItemGroup, ListGroup } from "@compozy/ui";

import { AgentCallsInspectorRow } from "./agent-calls-inspector-row";
import type { CallPayload } from "../types";

export interface CallDirectionSection {
  /** Rows loaded so far. */
  calls: readonly CallPayload[];
  /** The daemon's count for this direction. Undefined while the probe is in flight. */
  total: number | undefined;
  hasMore: boolean;
  onLoadMore: () => void;
  loadingMore?: boolean;
  error?: string | null;
  onRetry?: () => void;
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

function Section({
  label,
  section,
  direction,
  prunedSessionIds,
  onOpenCall,
}: {
  label: string;
  section: CallDirectionSection;
  direction: "made" | "received";
  prunedSessionIds: ReadonlySet<string>;
  onOpenCall: (callId: string) => void;
}) {
  const loaded = section.calls.length;
  return (
    <ListGroup
      data-testid={`agent-calls-panel-${direction}`}
      label={label}
      {...(section.total === undefined
        ? {}
        : {
            count: (
              <span data-testid={`agent-calls-panel-${direction}-count`}>{section.total}</span>
            ),
          })}
      headerProps={{ className: "border-line-soft bg-transparent px-0 py-1" }}
    >
      {section.error ? (
        <ActionResultBanner
          data-testid={`agent-calls-panel-${direction}-error`}
          title={`Couldn't load ${label.toLowerCase()} calls.`}
          description={section.error}
          tone="danger"
          actions={
            section.onRetry ? (
              <Button size="xs" type="button" variant="outline" onClick={section.onRetry}>
                Retry
              </Button>
            ) : null
          }
        />
      ) : null}

      {loaded === 0 && !section.error ? (
        <p className="py-1 text-form text-muted">
          {section.total === 0 ? "None yet." : "Loading…"}
        </p>
      ) : loaded > 0 ? (
        <ItemGroup className="gap-0">
          {section.calls.map(call => {
            const counterpartId =
              direction === "made" ? (call.child_session_id ?? "") : call.caller.id;
            return (
              <AgentCallsInspectorRow
                key={call.call_id}
                call={call}
                direction={direction}
                pruned={counterpartId !== "" && prunedSessionIds.has(counterpartId)}
                onOpenCall={onOpenCall}
              />
            );
          })}
        </ItemGroup>
      ) : null}

      {section.hasMore ? (
        <div className="flex items-center gap-2 pt-1">
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
    </ListGroup>
  );
}

const NO_PRUNED_SESSIONS: ReadonlySet<string> = new Set();

function hasPopulation(section: CallDirectionSection): boolean {
  return (section.total ?? section.calls.length) > 0;
}

function showDirection(section: CallDirectionSection, other: CallDirectionSection): boolean {
  if (section.error) return true;
  if (section.total === 0 && !hasPopulation(other)) return true;
  if (section.total === 0) return false;
  return true;
}

export function AgentCallsInspectorPanel({
  made,
  received,
  onOpenCall,
  prunedSessionIds = NO_PRUNED_SESSIONS,
  "data-testid": testId,
}: AgentCallsInspectorPanelProps) {
  const empty = made.total === 0 && received.total === 0;
  if (empty) {
    return <Empty data-testid={testId} title="No calls yet" />;
  }
  return (
    <div data-testid={testId} className="flex flex-col gap-4">
      {showDirection(made, received) ? (
        <Section
          label="Made"
          section={made}
          direction="made"
          prunedSessionIds={prunedSessionIds}
          onOpenCall={onOpenCall}
        />
      ) : null}
      {showDirection(received, made) ? (
        <Section
          label="Received"
          section={received}
          direction="received"
          prunedSessionIds={prunedSessionIds}
          onOpenCall={onOpenCall}
        />
      ) : null}
    </div>
  );
}
