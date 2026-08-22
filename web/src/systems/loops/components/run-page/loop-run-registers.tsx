import { useState } from "react";
import type { ReactNode } from "react";

import { Button } from "@compozy/ui";

import { buildGenerationHistory } from "../../lib/loop-generation-presentation";
import type { LoopNodeLifecycle } from "../../lib/loop-node-lifecycle";
import { resolveNodeVerbTarget } from "../../lib/loop-node-verb-target";
import type { LoopRosterRow } from "../../lib/loop-run-roster-table";
import { buildRosterTable, rosterRowKey } from "../../lib/loop-run-roster-table";
import type {
  LoopNodeSelection,
  LoopRunRegisters as LoopRunRegistersModel,
} from "../../lib/loop-run-registers-view";
import {
  LOOP_ROSTER_CONTINUATION_COMMAND,
  loopRosterReachNote,
  selectNodePanel,
} from "../../lib/loop-run-registers-view";
import type { LoopGraph } from "../../lib/loop-graph";
import type {
  LoopFanoutRollup,
  LoopRosterNode,
  LoopRunGeneration,
  LoopWatchEventsState,
} from "../../types";
import { LoopNodePanel } from "./inspect/loop-node-panel";
import { LoopNodeRoster } from "./inspect/loop-node-roster";
import { LoopRunDag } from "./inspect/loop-run-dag";
import type { LoopInspectLane } from "./inspect/loop-run-inspect-register";
import { LoopRunInspectRegister } from "./inspect/loop-run-inspect-register";
import { LoopGenerationHistory } from "./inspect/loop-generation-history";
import { LoopRunEventsLane } from "./inspect/loop-run-events-lane";
import type { LoopRunEventsRead } from "./inspect/loop-run-events-lane";
import { LoopRunWatch } from "./inspect/loop-run-watch";
import type { LoopRunRosterRead } from "./loop-run-page-body";

interface LoopRunRegistersProps {
  registers: LoopRunRegistersModel;
  nodes: readonly LoopRosterNode[];
  rollups: readonly LoopFanoutRollup[];
  graph: LoopGraph | null;
  generations: readonly LoopRunGeneration[];
  bestGeneration?: number | null;
  /** The run's own status; a terminal run with unsettled steps was interrupted. */
  runStatus?: string;
  /** The clock in-progress rows measure against; stories pin it for capture. */
  nowMs: number;
  isLive: boolean;
  isReconnecting: boolean;
  /** Pulls the next block of roster pages past the page's own budget. */
  onLoadMoreRoster?: () => void;
  isLoadingMoreRoster?: boolean;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Durable control truth per node; the verb rules read this, not the roster. */
  nodeLifecycles: readonly LoopNodeLifecycle[];
  /**
   * Which node is open. The page owns it because opening a node is what makes
   * that node's session worth asking about, and reads live at page level.
   */
  selection: LoopNodeSelection | null;
  onSelectionChange: (selection: LoopNodeSelection | null) => void;
  /** Sessions retention has removed; only the open node's is ever known. */
  prunedSessionIds?: ReadonlySet<string>;
  /**
   * Renders the verb cluster for one node. The register resolves *which* node
   * the selection means; the page owns what the daemon permits on it.
   */
  renderNodeActions?: (node: LoopNodeLifecycle) => ReactNode;
  onCompareGeneration?: (generation: number) => void;
  onForkGeneration?: (generation: number) => void;
  /** Controlled lane when a sibling surface, such as About, can open Inspect. */
  lane?: LoopInspectLane;
  onLaneChange?: (lane: LoopInspectLane) => void;
  /**
   * Whether the roster read has answered. Without it the lanes cannot tell a run
   * that reached no steps from one whose steps have not been read yet.
   */
  rosterRead?: LoopRunRosterRead;
  /**
   * The raw activity stream for the Events lane, read at `view=all`.
   *
   * Optional because the page can mount before that second read is wired; the
   * lane then borrows Story's notable projection and labels itself as such,
   * rather than presenting a filtered subset as the whole event log.
   */
  events?: Omit<LoopRunEventsRead, "view">;
  watchEvents?: LoopWatchEventsState | null;
}

/**
 * The operator register, wired.
 *
 * Which lane is showing and which round is filtered stay in-page state by
 * design: they describe where a person is looking right now, not something they
 * would want restored days later, and ADR-002 deliberately keeps them out of
 * `config.toml` and out of the URL. The open node is the exception — it decides
 * which session the page asks about, and asking is a read.
 */
export function LoopRunRegisters({
  registers,
  nodes,
  rollups,
  graph,
  generations,
  bestGeneration,
  runStatus,
  nowMs,
  isLive,
  isReconnecting,
  onLoadMoreRoster,
  isLoadingMoreRoster = false,
  open,
  onOpenChange,
  nodeLifecycles,
  selection,
  onSelectionChange,
  prunedSessionIds,
  renderNodeActions,
  onCompareGeneration,
  onForkGeneration,
  lane: controlledLane,
  onLaneChange,
  rosterRead,
  events,
  watchEvents,
}: LoopRunRegistersProps) {
  const [localLane, setLocalLane] = useState<LoopInspectLane>("graph");
  const lane = controlledLane ?? localLane;
  const changeLane = (nextLane: LoopInspectLane) => {
    if (controlledLane === undefined) setLocalLane(nextLane);
    onLaneChange?.(nextLane);
  };
  // `null` means "whatever round the run is on"; `"all"` is an explicit choice.
  // Following the run beats defaulting to every round, which makes two rows of
  // the same step indistinguishable until the reader asks for that.
  const [roundChoice, setRoundChoice] = useState<number | "all" | null>(null);
  const round = roundChoice === "all" ? null : (roundChoice ?? registers.round);

  const panel = selectNodePanel(nodes, selection, graph, prunedSessionIds);
  // A node whose lifecycle row exists keeps it — the roster does not model
  // pauses, waits or quarantine, and dropping them would offer verbs the daemon
  // refuses. A healthy node gets a stand-in built only from roster truth.
  const verbTarget = resolveNodeVerbTarget(selection, nodeLifecycles, nodes);
  const roster = buildRosterTable({
    nodes,
    rollups,
    graph,
    round,
    nowMs,
    isComplete: registers.reach.isComplete,
  });
  const generationRows = buildGenerationHistory({
    generations,
    nodes,
    runStatus,
    bestGeneration,
  });
  const reachNote = loopRosterReachNote(registers.reach);

  const selectRosterRow = (row: LoopRosterRow) =>
    onSelectionChange({
      nodeId: row.nodeId,
      itemIndex: row.itemIndex,
      generation: row.generation,
    });
  const renderRosterActions = renderNodeActions
    ? (row: LoopRosterRow) => {
        const target = resolveNodeVerbTarget(
          { nodeId: row.nodeId, itemIndex: row.itemIndex, generation: row.generation },
          nodeLifecycles,
          nodes
        );
        return target ? renderNodeActions(target) : null;
      }
    : undefined;

  const focusNote =
    lane === "graph" && registers.dag.focusReason ? registers.dag.focusReason : null;
  // Collected rather than nested, so "no notes at all" is a value the register
  // can act on. A fragment is always truthy, and passing one unconditionally
  // gave every uneventful read an empty bordered strip to explain.
  const footNotes: ReactNode[] = [];
  if (focusNote) {
    footNotes.push(
      <span data-testid="loop-run-inspect-focus" key="focus">
        {focusNote}
      </span>
    );
  }
  // A partial roster must never read as the whole run: the daemon returns oldest
  // round first, so a truncated read can be missing the round these views are
  // about. Saying so is not enough on its own — the reader also gets a way
  // through, here and at the command line.
  if (reachNote) {
    footNotes.push(
      <span
        data-testid={
          registers.reach.isTruncated ? "loop-run-inspect-truncated" : "loop-run-inspect-loading"
        }
        key="reach"
      >
        {reachNote}
      </span>
    );
  }
  if (registers.reach.isTruncated) {
    if (onLoadMoreRoster) {
      footNotes.push(
        <Button
          data-testid="loop-run-inspect-load-more"
          disabled={isLoadingMoreRoster}
          key="load-more"
          onClick={onLoadMoreRoster}
          size="sm"
          type="button"
          variant="ghost"
        >
          {isLoadingMoreRoster ? "Reading…" : "Read the rest"}
        </Button>
      );
    }
    footNotes.push(
      <span className="font-mono text-mono-id text-faint" key="continuation">
        {LOOP_ROSTER_CONTINUATION_COMMAND}
      </span>
    );
  }

  return (
    <LoopRunInspectRegister
      loadedEventCount={registers.loadedEventCount}
      foot={footNotes.length > 0 ? footNotes : undefined}
      generationCount={generations.length}
      isLive={isLive}
      isReconnecting={isReconnecting}
      lane={lane}
      loadedNodeCount={registers.reach.loadedCount}
      reach={registers.reach}
      onLaneChange={changeLane}
      onOpenChange={onOpenChange}
      open={open}
    >
      {lane === "graph" ? (
        <LoopRunDag
          dag={registers.dag}
          onSelect={onSelectionChange}
          read={rosterRead}
          selection={selection}
        />
      ) : null}
      {lane === "nodes" ? (
        <LoopNodeRoster
          onRoundChange={next => setRoundChoice(next === null ? "all" : next)}
          onSelect={selectRosterRow}
          renderActions={renderRosterActions}
          read={rosterRead}
          round={round}
          roster={roster}
          selectedKey={
            selection
              ? rosterRowKey({
                  generation: selection.generation,
                  node_id: selection.nodeId,
                  item_index: selection.itemIndex,
                })
              : null
          }
        />
      ) : null}
      {lane === "generations" ? (
        <LoopGenerationHistory
          onCompare={onCompareGeneration}
          onFork={onForkGeneration}
          rows={generationRows}
        />
      ) : null}
      {lane === "events" ? (
        <LoopRunEventsLane
          read={
            events
              ? { ...events, view: "all" }
              : {
                  beats: registers.beats,
                  view: "notable",
                  hasOlder: false,
                  isLoading: false,
                  isError: false,
                  isLoadingOlder: false,
                }
          }
        />
      ) : null}
      {panel && lane !== "events" ? (
        <div className="border-t border-line-soft p-4">
          <LoopNodePanel
            actions={verbTarget && renderNodeActions ? renderNodeActions(verbTarget) : undefined}
            panel={panel}
          />
        </div>
      ) : null}
      {watchEvents ? <LoopRunWatch watchEvents={watchEvents} /> : null}
    </LoopRunInspectRegister>
  );
}
