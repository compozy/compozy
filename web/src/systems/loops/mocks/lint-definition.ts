export interface MockLintIssue {
  node_id?: string;
  code: string;
  message: string;
  severity: "error" | "warning";
}

const PARENT_CLOSE_POLICIES = new Set(["terminate", "cancel", "abandon"]);
const WAIT_DISCRIMINATORS = ["for", "until", "event"] as const;
const DURATION_TOKEN_PATTERN = /(\d+(?:\.\d*)?|\.\d+)(ns|us|µs|μs|ms|s|m|h)/gy;
const DURATION_UNIT_MS: Record<string, number> = {
  ns: 1 / 1_000_000,
  us: 1 / 1_000,
  µs: 1 / 1_000,
  μs: 1 / 1_000,
  ms: 1,
  s: 1_000,
  m: 60_000,
  h: 3_600_000,
};

type RawRecord = Record<string, unknown>;

function record(value: unknown): RawRecord | null {
  return typeof value === "object" && value !== null && !Array.isArray(value)
    ? (value as RawRecord)
    : null;
}

function text(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}

/** Parses the Go duration subset the lifecycle keys use; `null` when unparseable. */
function durationMs(raw: string): number | null {
  const trimmed = raw.trim();
  if (trimmed === "0") return 0;
  const sign = trimmed.startsWith("-") ? -1 : 1;
  const unsigned = trimmed.startsWith("-") || trimmed.startsWith("+") ? trimmed.slice(1) : trimmed;
  if (unsigned === "") return null;
  DURATION_TOKEN_PATTERN.lastIndex = 0;
  let total = 0;
  let consumed = 0;
  for (
    let match = DURATION_TOKEN_PATTERN.exec(unsigned);
    match;
    match = DURATION_TOKEN_PATTERN.exec(unsigned)
  ) {
    if (match.index !== consumed) return null;
    total += Number(match[1]) * DURATION_UNIT_MS[match[2]];
    consumed = DURATION_TOKEN_PATTERN.lastIndex;
  }
  return consumed === unsigned.length ? sign * total : null;
}

/** Detects the first node caught in an edge cycle, mirroring the daemon acyclicity check. */
export function firstCycleNode(edges: { from: string; to: string }[]): string | null {
  const adjacency = new Map<string, string[]>();
  for (const edge of edges) {
    const list = adjacency.get(edge.from) ?? [];
    list.push(edge.to);
    adjacency.set(edge.from, list);
  }
  const visiting = new Set<string>();
  const done = new Set<string>();
  let found: string | null = null;
  const walk = (node: string) => {
    if (found || done.has(node)) return;
    visiting.add(node);
    for (const next of adjacency.get(node) ?? []) {
      if (visiting.has(next)) {
        found = next;
        return;
      }
      walk(next);
    }
    visiting.delete(node);
    done.add(node);
  };
  for (const node of adjacency.keys()) walk(node);
  return found;
}

/** `linter_effects.go`: exactly one valid emit or tool call, and emit never carries `with`. */
function lintEffectList(
  nodeId: string | undefined,
  effects: unknown,
  issues: MockLintIssue[]
): void {
  if (!Array.isArray(effects)) return;
  for (const entry of effects) {
    const effect = record(entry);
    const emit = effect ? record(effect.emit) : null;
    const tool = effect ? text(effect.tool) : "";
    const hasEmit = emit !== null;
    const hasTool = tool !== "";
    const emitInvalid =
      hasEmit && (text(emit?.kind) === "" || Object.prototype.hasOwnProperty.call(effect, "with"));
    if (hasEmit === hasTool || emitInvalid) {
      issues.push({
        ...(nodeId ? { node_id: nodeId } : {}),
        code: "effect_shape_invalid",
        message: "effect must declare exactly one valid emit or tool call",
        severity: "error",
      });
    }
  }
}

/** `linter_wait.go` lintWaitExpiry: `after` is required; no path is a WARNING, never a gate. */
function lintExpiry(
  nodeId: string,
  expiry: RawRecord | null,
  owner: string,
  issues: MockLintIssue[]
): void {
  if (!expiry) return;
  if (text(expiry.after) === "") {
    issues.push({
      node_id: nodeId,
      code: "wait_shape_invalid",
      message: `${owner} expires.after is required`,
      severity: "error",
    });
  }
  lintEffectList(nodeId, expiry.escalate, issues);
  const escalate = Array.isArray(expiry.escalate) ? expiry.escalate : [];
  if (escalate.length === 0 && text(expiry.route) === "") {
    issues.push({
      node_id: nodeId,
      code: "wait_expiry_without_path",
      message: `${owner} expiry has neither escalate effects nor route`,
      severity: "warning",
    });
  }
}

function lintNodeLifecycle(node: RawRecord, issues: MockLintIssue[]): void {
  const nodeId = String(node.id ?? "");
  const nodeClass = text(node.class);
  const kind = text(node.kind);

  const timeout = durationMs(text(node.timeout));
  const deadline = durationMs(text(node.deadline));
  if (timeout !== null && deadline !== null && timeout > deadline) {
    issues.push({
      node_id: nodeId,
      code: "timeout_exceeds_deadline",
      message: `timeout ${text(node.timeout)} exceeds deadline ${text(node.deadline)}`,
      severity: "error",
    });
  }

  const onError = record(node.on_error);
  if (onError && text(onError.route) !== "" && onError.allow_fail === true) {
    issues.push({
      node_id: nodeId,
      code: "error_route_conflict",
      message: "on_error.route and on_error.allow_fail are mutually exclusive",
      severity: "error",
    });
  }
  if (onError) lintEffectList(nodeId, onError.effects, issues);

  const resultContract = record(node.result_contract);
  if (resultContract && text(resultContract.failure_field) === "") {
    issues.push({
      node_id: nodeId,
      code: "result_contract_invalid",
      message: "result_contract.failure_field is required",
      severity: "error",
    });
  }

  for (const trigger of [
    "on_retry",
    "on_success",
    "on_pause",
    "on_timeout",
    "on_cancel",
    "on_quarantine",
  ]) {
    lintEffectList(nodeId, node[trigger], issues);
  }

  const parentClose = text(node.on_parent_close);
  if (parentClose !== "") {
    if (nodeClass !== "action" || kind !== "run-loop") {
      issues.push({
        node_id: nodeId,
        code: "parent_close_invalid",
        message: "on_parent_close is valid only on run-loop nodes",
        severity: "error",
      });
    } else if (!PARENT_CLOSE_POLICIES.has(parentClose)) {
      issues.push({
        node_id: nodeId,
        code: "parent_close_invalid",
        message: `on_parent_close "${parentClose}" is not in the closed enum`,
        severity: "error",
      });
    }
  }

  lintExpiry(nodeId, record(node.expires), "gate", issues);

  if (nodeClass === "control" && kind === "wait") {
    const params = record(node.params) ?? {};
    const declared = WAIT_DISCRIMINATORS.filter(key => {
      const value = params[key];
      return typeof value === "string"
        ? value.trim() !== ""
        : value !== undefined && value !== null;
    });
    if (declared.length !== 1) {
      issues.push({
        node_id: nodeId,
        code: "wait_shape_invalid",
        message: "wait params must declare exactly one of for, until, or event",
        severity: "error",
      });
    }
    lintExpiry(nodeId, record(params.expires), "wait", issues);
  }
}

/**
 * Lints a posted definition the way the shared Go linter does. `graph` carries nodes/edges;
 * `contract` carries the seven terminal effect lists.
 */
export function lintDefinition(definition?: {
  graph?: { nodes?: unknown[]; edges?: unknown[] };
  contract?: unknown;
}): MockLintIssue[] {
  const issues: MockLintIssue[] = [];
  const graph = definition?.graph;
  const nodes = Array.isArray(graph?.nodes) ? (graph.nodes as RawRecord[]) : [];
  for (const node of nodes) {
    const fanOut = node.max_fan_out;
    if (node.kind === "fan-out" && (typeof fanOut !== "number" || fanOut <= 0)) {
      issues.push({
        node_id: String(node.id),
        code: "fan_out_unbounded",
        message: "fan-out must declare collection and a positive max_fan_out.",
        severity: "error",
      });
    }
    if (node.kind === "route" && text(node.default) === "") {
      issues.push({
        node_id: String(node.id),
        code: "route_default_missing",
        message: "route node must declare a default target",
        severity: "error",
      });
    }
    lintNodeLifecycle(node, issues);
  }
  const contract = record(definition?.contract);
  if (contract) {
    for (const terminal of [
      "on_done",
      "on_noop",
      "on_blocked",
      "on_failed",
      "on_exhausted",
      "on_stalled",
      "on_canceled",
    ]) {
      lintEffectList(undefined, contract[terminal], issues);
    }
  }
  const edges = Array.isArray(graph?.edges)
    ? (graph.edges as { from: string; to: string }[]).filter(edge => edge?.from && edge?.to)
    : [];
  const cycleNode = firstCycleNode(edges);
  if (cycleNode) {
    issues.push({
      node_id: cycleNode,
      code: "cycle_detected",
      message: `The graph is not acyclic: ${cycleNode} is part of a cycle.`,
      severity: "error",
    });
  }
  return issues;
}
