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
import type { LoopFanoutRollup, LoopRosterNode } from "../../types";
import { LoopNodePanel } from "./inspect/loop-node-panel";
import { LoopNodeRoster } from "./inspect/loop-node-roster";
import { LoopRunDag } from "./inspect/loop-run-dag";
import type { LoopInspectLane } from "./inspect/loop-run-inspect-register";
import { LoopRunInspectRegister } from "./inspect/loop-run-inspect-register";
import { LoopGenerationHistory } from "./inspect/loop-generation-history";
import { LoopRunEventsLane } from "./inspect/loop-run-events-lane";
import type { LoopRunGeneration } from "../../types";

interface LoopRunRegistersProps {
  registers: LoopRunRegistersModel;
  nodes: readonly LoopRosterNode[];
  rollups: readonly LoopFanoutRollup[];
  graph: LoopGraph | null;
  generations: readonly LoopRunGeneration[];
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
}: LoopRunRegistersProps) {
  const [lane, setLane] = useState<LoopInspectLane>("graph");
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
  const roster = buildRosterTable({ nodes, rollups, graph, round, nowMs });
  const generationRows = buildGenerationHistory({ generations, nodes, runStatus });
  const reachNote = loopRosterReachNote(registers.reach);

  const selectRosterRow = (row: LoopRosterRow) =>
    onSelectionChange({
      nodeId: row.nodeId,
      itemIndex: row.itemIndex,
      generation: row.generation,
    });

  return (
    <LoopRunInspectRegister
      eventCount={registers.eventCount}
      foot={
        <>
          {lane === "graph" && registers.dag.focusReason ? (
            <span data-testid="loop-run-inspect-focus">{registers.dag.focusReason}</span>
          ) : null}
          {/* A partial roster must never read as the whole run: the daemon
              returns oldest round first, so a truncated read can be missing the
              round these views are about. Saying so is not enough on its own —
              the reader also gets a way through, here and at the command line. */}
          {reachNote ? (
            <span
              data-testid={
                registers.reach.isTruncated
                  ? "loop-run-inspect-truncated"
                  : "loop-run-inspect-loading"
              }
            >
              {reachNote}
            </span>
          ) : null}
          {registers.reach.isTruncated ? (
            <>
              {onLoadMoreRoster ? (
                <Button
                  data-testid="loop-run-inspect-load-more"
                  disabled={isLoadingMoreRoster}
                  onClick={onLoadMoreRoster}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  {isLoadingMoreRoster ? "Reading…" : "Read the rest"}
                </Button>
              ) : null}
              <span className="font-mono text-mono-id text-faint">
                {LOOP_ROSTER_CONTINUATION_COMMAND}
              </span>
            </>
          ) : null}
        </>
      }
      generationCount={generations.length}
      isLive={isLive}
      isReconnecting={isReconnecting}
      lane={lane}
      nodeCount={registers.nodeCount}
      onLaneChange={setLane}
      onOpenChange={onOpenChange}
      open={open}
    >
      {lane === "graph" ? (
        <LoopRunDag
          dag={registers.dag}
          onSelect={onSelectionChange}
          selectedNodeId={selection?.nodeId ?? null}
        />
      ) : null}
      {lane === "nodes" ? (
        <LoopNodeRoster
          onRoundChange={next => setRoundChoice(next === null ? "all" : next)}
          onSelect={selectRosterRow}
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
      {lane === "events" ? <LoopRunEventsLane beats={registers.beats} /> : null}
      {panel && lane !== "events" ? (
        <div className="border-t border-line-soft p-4">
          <LoopNodePanel
            actions={verbTarget && renderNodeActions ? renderNodeActions(verbTarget) : undefined}
            panel={panel}
          />
        </div>
      ) : null}
    </LoopRunInspectRegister>
  );
}
