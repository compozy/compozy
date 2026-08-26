import { type LoopQuarantineEntry, readQuarantineEntry } from "./loop-quarantine-entry";
import { humanizeLoopNodeId } from "./loop-node-labels";
import type {
  LoopControlProvenance,
  LoopNodeControl,
  LoopNodeWait,
  LoopRunGeneration,
  LoopRunGenerationOutput,
} from "../types";

/**
 * The per-node lifecycle projection (SD-007 daemon truth). It joins the three
 * durable sources the run detail already returns — `node_controls[]` (pause,
 * quarantine, attention, cancel), `waits[]` (timer/event/approval cells), and the
 * latest generation's `outputs[]` (attempt counter + next attempt) — into one
 * read model per authored node.
 *
 * Nothing here infers a state: a node is retrying only because the daemon wrote
 * `next_attempt_at`, paused only because `control.paused` is true, waiting only
 * because an unclaimed wait cell exists. A node the payload says nothing about
 * gets no lifecycle row at all.
 */

/** The parked/at-risk vocabulary the run page renders, in precedence order. */
export type LoopNodeLifecycleState =
  | "quarantined"
  | "canceling"
  | "canceled"
  | "paused"
  | "waiting"
  | "retrying";

/** Parked states suspend the node's clock; they never count as failing work. */
const PARKED_STATES = new Set<LoopNodeLifecycleState>(["quarantined", "paused", "waiting"]);

export function isParkedLifecycleState(state: LoopNodeLifecycleState | null): boolean {
  return state !== null && PARKED_STATES.has(state);
}

export interface LoopNodeWaitView {
  nodeId: string;
  generation: number;
  itemIndex: number;
  kind: string;
  claimState: string;
  resumeAt?: string;
  nextEscalationAt?: string;
  escalationCursor: number;
  admissionFailures: number;
  /** Seconds the cell has been open, computed by the daemon at read time. */
  ageSeconds: number;
  createdAt: string;
  /** The declared match condition for an event wait, when the daemon recorded one. */
  expect: unknown;
}

export interface LoopNodeLifecycle {
  nodeId: string;
  /** `fix_batches` → "fix batches" — the plain-language label used in copy. */
  label: string;
  state: LoopNodeLifecycleState | null;
  parked: boolean;
  paused: boolean;
  pauseProvenance: LoopControlProvenance | null;
  quarantined: boolean;
  quarantinedAt: string | null;
  quarantineEntry: LoopQuarantineEntry | null;
  attentionFlag: string;
  attentionReason: string;
  /** The parked producer a dependency-flagged consumer routes to for repair. */
  attentionProducerNodeId: string;
  cancelState: string;
  cancelProvenance: LoopControlProvenance | null;
  lastEvidenceAt: string | null;
  deathResumeStreak: number;
  revision: number;
  waits: LoopNodeWaitView[];
  /** Attempt counter from the latest generation output; absent when never retried. */
  attempt: number | null;
  nextAttemptAt: string | null;
  failureClass: string;
  disposition: string;
  outputStatus: string;
  generation: number;
  /** Fan-out item from the latest output; null when the node has no item-scoped cell. */
  itemIndex: number | null;
  /** ACP session bound to the node's latest cell run; null when none bound yet. */
  sessionId: string | null;
}

function provenance(value: LoopControlProvenance | null | undefined): LoopControlProvenance | null {
  return value ?? null;
}

function waitView(wait: LoopNodeWait): LoopNodeWaitView {
  return {
    nodeId: wait.node_id,
    generation: wait.generation,
    itemIndex: wait.item_index,
    kind: wait.kind,
    claimState: wait.claim_state,
    resumeAt: wait.resume_at ?? undefined,
    nextEscalationAt: wait.next_escalation_at ?? undefined,
    escalationCursor: wait.escalation_cursor,
    admissionFailures: wait.admission_failures,
    ageSeconds: wait.age_seconds,
    createdAt: wait.created_at,
    expect: wait.expect,
  };
}

/** A wait still holds the node only while nobody has resumed or claimed it. */
export function isOpenWait(wait: LoopNodeWaitView): boolean {
  return wait.claimState === "waiting" || wait.claimState === "intervention_required";
}

/** The newest output for one node in the latest generation the run reports. */
function latestOutputs(
  generations: readonly LoopRunGeneration[] | undefined
): Map<string, LoopRunGenerationOutput> {
  const byNode = new Map<string, LoopRunGenerationOutput>();
  if (!generations || generations.length === 0) return byNode;
  const latest = generations.reduce((max, gen) => Math.max(max, gen.generation), 0);
  for (const generation of generations) {
    if (generation.generation !== latest) continue;
    for (const output of generation.outputs) {
      const previous = byNode.get(output.node_id);
      // Highest attempt wins: a fan-out node has one output per branch, and the
      // lifecycle row summarizes the branch that has been retried the most.
      if (!previous || (output.attempt ?? 0) >= (previous.attempt ?? 0)) {
        byNode.set(output.node_id, output);
      }
    }
  }
  return byNode;
}

/**
 * Resolves the single state the node chrome renders. Quarantine and cancellation
 * outrank a pause because they are terminal for the current episode; a pause
 * outranks a wait because the operator's park is the reason nothing moves.
 */
function lifecycleState(
  control: LoopNodeControl | undefined,
  openWaits: number,
  output: LoopRunGenerationOutput | undefined
): LoopNodeLifecycleState | null {
  if (control?.quarantined) return "quarantined";
  const cancelState = control?.cancel_state ?? "";
  if (cancelState === "canceled") return "canceled";
  if (cancelState !== "") return "canceling";
  if (control?.paused) return "paused";
  if (openWaits > 0) return "waiting";
  if (output?.next_attempt_at) return "retrying";
  return null;
}

export interface LoopNodeLifecycleInput {
  controls: readonly LoopNodeControl[] | undefined;
  waits: readonly LoopNodeWait[] | undefined;
  generations: readonly LoopRunGeneration[] | undefined;
  runGeneration: number;
}

/**
 * Projects one lifecycle row per node the daemon reports control, wait, or retry
 * state for. Rows are ordered by node id so the run page and the inventory agree
 * on presentation order without inventing a ranking the daemon does not publish.
 */
export function projectNodeLifecycles(input: LoopNodeLifecycleInput): LoopNodeLifecycle[] {
  const outputs = latestOutputs(input.generations);
  const waitsByNode = new Map<string, LoopNodeWaitView[]>();
  for (const wait of input.waits ?? []) {
    const list = waitsByNode.get(wait.node_id) ?? [];
    list.push(waitView(wait));
    waitsByNode.set(wait.node_id, list);
  }
  const controlsByNode = new Map<string, LoopNodeControl>();
  for (const control of input.controls ?? []) controlsByNode.set(control.node_id, control);

  const nodeIds = new Set<string>([...controlsByNode.keys(), ...waitsByNode.keys()]);
  for (const [nodeId, output] of outputs) {
    if (output.next_attempt_at) nodeIds.add(nodeId);
  }

  const rows: LoopNodeLifecycle[] = [];
  for (const nodeId of [...nodeIds].sort()) {
    const control = controlsByNode.get(nodeId);
    const waits = waitsByNode.get(nodeId) ?? [];
    const output = outputs.get(nodeId);
    const openWaits = waits.filter(isOpenWait).length;
    const state = lifecycleState(control, openWaits, output);
    // A node with no control record, no open wait, and no scheduled retry has no
    // lifecycle story — skip it instead of rendering a row that says nothing.
    if (state === null && !control && waits.length === 0) continue;
    rows.push({
      nodeId,
      label: humanizeLoopNodeId(nodeId),
      state,
      parked: isParkedLifecycleState(state),
      paused: control?.paused === true,
      pauseProvenance: provenance(control?.pause_provenance),
      quarantined: control?.quarantined === true,
      quarantinedAt: control?.quarantined_at ?? null,
      quarantineEntry: readQuarantineEntry(control?.quarantine_entry),
      attentionFlag: control?.attention_flag ?? "",
      attentionReason: control?.attention_reason ?? "",
      attentionProducerNodeId: control?.attention_producer_node_id ?? "",
      cancelState: control?.cancel_state ?? "",
      cancelProvenance: provenance(control?.cancel_provenance),
      lastEvidenceAt: control?.last_evidence_at ?? null,
      deathResumeStreak: control?.death_resume_streak ?? 0,
      revision: control?.revision ?? 0,
      waits,
      attempt: output?.attempt ?? null,
      nextAttemptAt: output?.next_attempt_at ?? null,
      failureClass: output?.failure_class ?? "",
      disposition: output?.disposition ?? "",
      outputStatus: output?.status ?? "",
      generation: Number(output?.generation ?? input.runGeneration),
      itemIndex: typeof output?.item_index === "number" ? output.item_index : null,
      sessionId: output?.session_id ? output.session_id : null,
    });
  }
  return rows;
}

/**
 * ACP session per node from the latest generation's cells — the run page's
 * one-click path into the live agent conversation.
 */
/** Nodes holding at least one open wait cell. */
export function waitingNodes(rows: readonly LoopNodeLifecycle[]): LoopNodeLifecycle[] {
  return rows.filter(row => row.waits.some(isOpenWait));
}
