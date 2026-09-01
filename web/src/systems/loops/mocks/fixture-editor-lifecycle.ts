import type { LoopDetail } from "../types";
import { loopDetailByName } from "./fixtures";

/** Lifecycle editor fixtures derived from the workspace-only quality-gate detail. */

type RawNode = Record<string, unknown>;
type RawGraph = { nodes: RawNode[]; edges: unknown[] };

const qualityGate = loopDetailByName.get("quality-gate-demo")!;

function graphOf(detail: LoopDetail): RawGraph {
  return detail.definition.graph as unknown as RawGraph;
}

function withGraph(detail: LoopDetail, nodes: RawNode[]): LoopDetail {
  const graph = graphOf(detail);
  return {
    ...detail,
    definition: {
      ...detail.definition,
      graph: { ...graph, nodes } as unknown as LoopDetail["definition"]["graph"],
    },
  };
}

/** Replaces one node by id, leaving every other node verbatim. */
function withNode(detail: LoopDetail, id: string, patch: RawNode): LoopDetail {
  return withGraph(
    detail,
    graphOf(detail).nodes.map(node => (node.id === id ? { ...node, ...patch } : node))
  );
}

/** Appends a node and wires it after `after`, so it is reachable for the linter. */
function withAppendedNode(detail: LoopDetail, node: RawNode, after: string): LoopDetail {
  const graph = graphOf(detail);
  return {
    ...detail,
    definition: {
      ...detail.definition,
      graph: {
        ...graph,
        nodes: [...graph.nodes, node],
        edges: [...graph.edges, { from: after, to: node.id }],
      } as unknown as LoopDetail["definition"]["graph"],
    },
  };
}

/** Reliability and node triggers authored on the run-agent node. */
export const LIFECYCLE_EXECUTE_TASK: RawNode = {
  timeout: "45m",
  deadline: "2h",
  retry: {
    max_attempts: 3,
    backoff: { base: "30s", max: "5m" },
    non_retryable: ["tool_invalid_input"],
  },
  result_contract: { failure_field: "err", message_field: "err.message" },
  on_error: {
    allow_fail: true,
    effects: [{ tool: "compozy__network_send", with: { channel: "ops" } }],
  },
  on_retry: [{ emit: { kind: "task_retrying" } }],
  on_quarantine: [{ tool: "compozy__network_send", with: { channel: "ops" } }],
};

/** A `wait` control node declaring exactly one discriminator plus an expiry path. */
export const WAIT_NODE: RawNode = {
  id: "await_deploy_ack",
  class: "control",
  kind: "wait",
  params: {
    event: { kind: "task_status_changed", filter: "event.payload.to_status == 'completed'" },
    expect: { type: "object", required: ["deploy_id"] },
    ahead_arrival: "consume_on_entry",
    expires: { after: "24h", route: "release_notes" },
  },
};

/** The same wait node with an expiry that has neither escalate nor route — a WARNING only. */
export const WAIT_NODE_WITHOUT_EXPIRY_PATH: RawNode = {
  ...WAIT_NODE,
  params: { ...(WAIT_NODE.params as RawNode), expires: { after: "24h" } },
};

/** A run-loop action declaring the parent-close policy. */
export const RUN_LOOP_NODE: RawNode = {
  id: "release_notes",
  class: "action",
  kind: "run-loop",
  params: { loop: "release-notes", mode: "await" },
  on_parent_close: "terminate",
};

/** A custom loop whose run-agent node carries the whole authored failure contract. */
export const lifecycleAuthoredDetail: LoopDetail = withNode(
  qualityGate,
  "execute_task",
  LIFECYCLE_EXECUTE_TASK
);

export const contractTerminalsDetail: LoopDetail = {
  ...qualityGate,
  definition: {
    ...qualityGate.definition,
    contract: {
      ...qualityGate.definition.contract,
      on_failed: [{ tool: "compozy__network_send", with: { channel: "ops" } }],
      on_canceled: [{ emit: { kind: "delivery_canceled" } }],
    },
  },
};

/** A custom loop carrying the wait control node. */
export const waitNodeDetail: LoopDetail = withAppendedNode(qualityGate, WAIT_NODE, "collect");

/** The wait node whose expiry declares no path — the canonical warning-only verdict. */
export const waitWarningDetail: LoopDetail = withAppendedNode(
  qualityGate,
  WAIT_NODE_WITHOUT_EXPIRY_PATH,
  "collect"
);

/** A custom loop carrying the run-loop node with `on_parent_close`. */
export const runLoopNodeDetail: LoopDetail = withAppendedNode(
  qualityGate,
  RUN_LOOP_NODE,
  "collect"
);

/**
 * A read-only definition: immutable, Publish illegal, Fork the only mutation. `marketplace` is
 * a real member of the shipped `LoopSource` enum (`marketplace | user | additional |
 * workspace`), so the fixture and the strip copy name a source the runtime actually reports.
 * `version` drops to 1 so a test can prove a rejected publish never advances the pill.
 */
export const readOnlySourceDetail: LoopDetail = {
  ...qualityGate,
  source: "marketplace",
  version: 1,
  definition: {
    ...qualityGate.definition,
    meta: { ...qualityGate.definition.meta, version: 1 },
  },
};

/** Fixture combining reliability, wait, parent-close, and terminal-reaction fields. */
export const fullLifecycleDetail: LoopDetail = (() => {
  const withNodes = withAppendedNode(
    withAppendedNode(lifecycleAuthoredDetail, WAIT_NODE, "collect"),
    RUN_LOOP_NODE,
    "await_deploy_ack"
  );
  return {
    ...withNodes,
    definition: {
      ...withNodes.definition,
      contract: contractTerminalsDetail.definition.contract,
    },
  };
})();

/**
 * A definition that trips one blocking error (fan-out lacks a positive bound) AND one warning
 * (`wait_expiry_without_path`) at the same time — the dock's severity-split state.
 */
export const lintErrorAndWarningDetail: LoopDetail = withNode(waitWarningDetail, "implement", {
  max_fan_out: 0,
});

/** The issue list a publish 422 returns in the rejected-chrome fixtures. */
export const PUBLISH_REJECTED_ISSUES = [
  {
    node_id: "implement",
    code: "fan_out_unbounded",
    message: "fan-out must declare collection and a positive max_fan_out.",
    severity: "error" as const,
  },
  {
    node_id: "execute_task",
    code: "error_route_conflict",
    message: "on_error.route and on_error.allow_fail are mutually exclusive",
    severity: "error" as const,
  },
];
