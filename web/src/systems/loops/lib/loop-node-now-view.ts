import type { LoopNodeRetrySchedule } from "./loop-events";
import { nodeRetryMaxAttempts, type LoopGraph } from "./loop-graph";
import { isOpenWait, type LoopNodeLifecycle, type LoopNodeWaitView } from "./loop-node-lifecycle";

/**
 * The lifecycle half of "Happening now" (VC-R1/VC-R2). A node the daemon parked
 * or scheduled a retry for gets its own line inside the existing card anatomy —
 * headline, one plain-language sentence, an optional state chip, and the mono
 * identity trail — rather than a parallel card of its own.
 *
 * Every clause is conditional on a payload field: no attempt ceiling unless the
 * author declared `retry.max_attempts`, no due time unless the daemon wrote
 * `next_attempt_at`, no provenance unless the control carries it.
 */

export interface LoopNodeNowLine {
  nodeId: string;
  /** `paused`, `retrying`, … — drives the dot treatment and the trailing label. */
  state: string;
  headline: string;
  body: string;
  /** Warning-tinted chip (`attempt 2 of 3 · next 14:33:41`); omitted when empty. */
  chip: string | null;
  /** Provenance line — who parked it and why, verbatim from the control. */
  provenance: string | null;
  /** Mono identity trail (DESIGN-LESSONS L9). */
  micro: string;
}

function actorLabel(kind: string, id: string): string {
  if (kind === "user" || kind === "operator") return id === "" ? "you" : `you (${id})`;
  if (kind === "" && id === "") return "";
  // Either half can be absent on the wire; join only what is actually there.
  return [kind, id].filter(part => part !== "").join(" ");
}

/** `2026-08-03T14:33:41Z` → `14:33:41`, the artboard's due-time form. */
function clockTime(value: string): string {
  const parsed = Date.parse(value);
  if (Number.isNaN(parsed)) return "";
  return new Date(parsed).toLocaleTimeString([], {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });
}

function waitClause(wait: LoopNodeWaitView): string {
  switch (wait.kind) {
    case "timer":
      return wait.resumeAt
        ? `Sleeping until ${clockTime(wait.resumeAt)} — it resumes by itself.`
        : "Sleeping on a timer — it resumes by itself.";
    case "event":
      return "Waiting for a matching event. A lane's wait is its own; nothing else is held up.";
    case "approval_escalation":
      return "Waiting for your decision. Deciding at any point resolves it.";
    default:
      return "Waiting. A lane's wait is its own; nothing else is held up.";
  }
}

function retryLine(
  node: LoopNodeLifecycle,
  graph: LoopGraph | null,
  schedule: LoopNodeRetrySchedule | undefined
): LoopNodeNowLine {
  const attempt = schedule?.attempt ?? node.attempt ?? 0;
  const max = nodeRetryMaxAttempts(graph, node.nodeId);
  const attemptLabel = max === undefined ? `attempt ${attempt}` : `attempt ${attempt} of ${max}`;
  const nextAttemptAt = schedule?.nextAttemptAt ?? node.nextAttemptAt ?? "";
  const due = nextAttemptAt === "" ? "" : clockTime(nextAttemptAt);
  const failureClass = schedule?.failureClass || node.failureClass;
  const classClause =
    failureClass === ""
      ? ""
      : ` Only ${failureClass}-class failures retry on their own — everything else asks or parks.`;
  return {
    nodeId: node.nodeId,
    state: "retrying",
    headline: `${node.label} is retrying — ${attemptLabel}`,
    body:
      due === ""
        ? `Nothing to do: the retry runs by itself.${classClause}`
        : `Nothing to do: the retry runs by itself at ${due}.${classClause}`,
    chip: due === "" ? attemptLabel : `${attemptLabel} · next ${due}`,
    provenance: null,
    micro: [
      failureClass === "" ? null : `failure_class ${failureClass}`,
      node.disposition === "" ? null : node.disposition,
      `${node.nodeId} · gen ${node.generation}`,
    ]
      .filter(Boolean)
      .join(" · "),
  };
}

function pausedLine(node: LoopNodeLifecycle): LoopNodeNowLine {
  const provenance = node.pauseProvenance;
  const actor = provenance ? actorLabel(provenance.actor_kind, provenance.actor_id) : "";
  const byRule = provenance?.rule_id ? `rule ${provenance.rule_id}` : "";
  const by = byRule || actor;
  const requestedAt = provenance ? clockTime(provenance.requested_at) : "";
  return {
    nodeId: node.nodeId,
    state: "paused",
    headline: `${node.label} is paused`,
    body: `${by === "" ? "Parked" : `Paused by ${by}`} at a safe point — the rest of the run keeps working.`,
    chip: null,
    provenance: provenance
      ? [
          `paused by ${by || "unknown"}`,
          provenance.reason ? `reason “${provenance.reason}”` : null,
          requestedAt === "" ? null : `at ${requestedAt}`,
        ]
          .filter(Boolean)
          .join(" · ")
      : null,
    micro: `node_controls.paused true · ${node.nodeId} · gen ${node.generation}`,
  };
}

function waitingLine(node: LoopNodeLifecycle): LoopNodeNowLine | null {
  const wait = node.waits.find(isOpenWait);
  if (!wait) return null;
  return {
    nodeId: node.nodeId,
    state: "waiting",
    headline: `${node.label} is waiting on ${wait.kind === "timer" ? "a timer" : wait.kind === "event" ? "an event" : "a decision"}`,
    body: waitClause(wait),
    chip: null,
    provenance: null,
    micro: [
      wait.kind,
      wait.resumeAt ? `resumes ${clockTime(wait.resumeAt)}` : null,
      `${node.nodeId}[${wait.itemIndex}] · gen ${wait.generation}`,
    ]
      .filter(Boolean)
      .join(" · "),
  };
}

function quarantinedLine(node: LoopNodeLifecycle): LoopNodeNowLine {
  const attempts = node.quarantineEntry?.attemptCount ?? 0;
  return {
    nodeId: node.nodeId,
    state: "quarantined",
    headline: `${node.label} is set aside`,
    body:
      (attempts > 0 ? `Quarantined after ${attempts} attempts — ` : "Quarantined — ") +
      "its clock is suspended and no attempts are being spent. The rest of the run keeps going; " +
      "requeue it from the entry once it is repaired.",
    chip: null,
    provenance: node.quarantinedAt ? `quarantined ${clockTime(node.quarantinedAt)}` : null,
    micro: `node_controls.quarantined true · ${node.nodeId} · gen ${node.generation}`,
  };
}

function cancelingLine(node: LoopNodeLifecycle): LoopNodeNowLine {
  const provenance = node.cancelProvenance;
  const actor = provenance ? actorLabel(provenance.actor_kind, provenance.actor_id) : "";
  return {
    nodeId: node.nodeId,
    state: "canceling",
    headline: `${node.label} is winding down`,
    body: `Cancellation is in progress${actor === "" ? "" : ` — requested by ${actor}`}. Its in-flight work drains before the lane closes.`,
    chip: null,
    provenance: null,
    micro: `node_controls.cancel_state ${node.cancelState} · ${node.nodeId} · gen ${node.generation}`,
  };
}

/**
 * The lifecycle lines the "Happening now" card renders, in daemon precedence
 * order. A node with no declared lifecycle state contributes nothing.
 */
export function buildNodeNowLines(
  nodes: readonly LoopNodeLifecycle[],
  graph: LoopGraph | null,
  retrySchedules: Readonly<Record<string, LoopNodeRetrySchedule>>
): LoopNodeNowLine[] {
  const lines: LoopNodeNowLine[] = [];
  for (const node of nodes) {
    switch (node.state) {
      case "retrying":
        lines.push(retryLine(node, graph, retrySchedules[node.nodeId]));
        break;
      case "paused":
        lines.push(pausedLine(node));
        break;
      case "waiting": {
        const line = waitingLine(node);
        if (line) lines.push(line);
        break;
      }
      case "quarantined":
        lines.push(quarantinedLine(node));
        break;
      case "canceling":
        lines.push(cancelingLine(node));
        break;
      // `canceled` is settled, not happening — it belongs to the story, not here.
      default:
        break;
    }
  }
  return lines;
}
