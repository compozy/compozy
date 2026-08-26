import type { EditorNode } from "./codec";
import type { LoopValidationIssue, ValidateLoopResult } from "../types";

export type LoopInvariantKey =
  | "acyclicity"
  | "reachability"
  | "termination"
  | "fan_out"
  | "references"
  | "routing"
  | "human_request";

export type LoopInvariantStatus = "pass" | "fail";

const CODE_KEYWORDS: { key: LoopInvariantKey; keywords: string[] }[] = [
  { key: "acyclicity", keywords: ["cycle", "acyclic"] },
  { key: "reachability", keywords: ["unreachable", "reachab"] },
  { key: "termination", keywords: ["terminat", "no_terminal", "nonterminating"] },
  {
    key: "references",
    keywords: ["reference", "unresolvable", "condition_not_bool", "item_outside_fanout", "node_id"],
  },

  { key: "human_request", keywords: ["ask_", "review_", "responder_"] },
  {
    key: "routing",
    keywords: ["route_", "eval_error_policy", "error_route"],
  },

  { key: "fan_out", keywords: ["fan_out", "fanout", "strategy_", "iteration_name"] },
];

/** Classifies a lint code to its invariant chip, or `null` when unattributable. */
export function classifyInvariant(code: string): LoopInvariantKey | null {
  const normalized = code.toLowerCase();
  for (const { key, keywords } of CODE_KEYWORDS) {
    if (keywords.some(keyword => normalized.includes(keyword))) return key;
  }
  return null;
}

/**
 * Whether an issue blocks Publish. Anything not explicitly a warning/info is treated as a
 * blocking error, so an unknown severity fails safe (Publish stays disabled) rather than
 * shipping. Non-blocking issues (warnings) must not be rendered as a 422 gate.
 */
export function isBlockingIssue(issue: LoopValidationIssue): boolean {
  const severity = issue.severity?.toLowerCase();
  return severity !== "warning" && severity !== "warn" && severity !== "info";
}

export interface LoopLintState {
  issues: LoopValidationIssue[];
  /** Per-node issues keyed by `node_id`, for canvas badges + inspector highlight. */
  byNode: Map<string, LoopValidationIssue[]>;
  /** Issues that carry no `node_id` (whole-graph problems). */
  general: LoopValidationIssue[];
  invariants: Record<LoopInvariantKey, LoopInvariantStatus>;
  /** True when at least one blocking error exists — Publish is disabled. */
  hasBlockingErrors: boolean;
  errorCount: number;
  /** Non-blocking issues (`wait_expiry_without_path`, `effect_tool_unknown`, …). */
  warningCount: number;
  /**
   * True only after a real daemon verdict. The pre-validation state is `false` so the UI
   * shows a neutral "not yet validated" state instead of asserting a pass the daemon has
   * not confirmed (truthful-UI: daemon truth wins).
   */
  validated: boolean;
}

/** The pre-validation state (no daemon verdict yet): neutral, not a claimed pass. */
export function emptyLintState(): LoopLintState {
  return {
    issues: [],
    byNode: new Map(),
    general: [],
    invariants: {
      acyclicity: "pass",
      reachability: "pass",
      termination: "pass",
      fan_out: "pass",
      references: "pass",
      routing: "pass",
      human_request: "pass",
    },
    hasBlockingErrors: false,
    errorCount: 0,
    warningCount: 0,
    validated: false,
  };
}

/** The dock's header state; zero counts are omitted. */
export type LoopLintDockState = "pending" | "unavailable" | "clean" | "issues";

export interface LoopLintDockCounters {
  state: LoopLintDockState;
  /** Present only when greater than zero. */
  errors?: number;
  /** Present only when greater than zero. */
  warnings?: number;
}

/**
 * Maps a verdict onto the dock header. Clean verdicts carry no counter; issue verdicts expose
 * only non-zero severities.
 */
export function lintDockCounters(
  lint: LoopLintState,
  validateFailed: boolean
): LoopLintDockCounters {
  if (!lint.validated) return { state: validateFailed ? "unavailable" : "pending" };
  const counters: LoopLintDockCounters = {
    state: lint.issues.length === 0 ? "clean" : "issues",
  };
  if (lint.errorCount > 0) counters.errors = lint.errorCount;
  if (lint.warningCount > 0) counters.warnings = lint.warningCount;
  return counters;
}

/**
 * Builds the editor lint state from a validate/publish verdict. Groups issues by node,
 * flips the invariant chips a blocking error attributes to, and counts blocking errors.
 */
export function buildLintState(result: ValidateLoopResult | null): LoopLintState {
  const state = emptyLintState();
  // A result (even a passing one) is a real daemon verdict; a null result stays neutral.
  state.validated = result !== null;
  const issues = result?.errors ?? [];
  if (issues.length === 0) return state;
  state.issues = [...issues];
  for (const issue of issues) {
    const blocking = isBlockingIssue(issue);
    if (blocking) state.errorCount += 1;
    else state.warningCount += 1;
    const nodeId = issue.node_id?.trim();
    if (nodeId) {
      const bucket = state.byNode.get(nodeId);
      if (bucket) bucket.push(issue);
      else state.byNode.set(nodeId, [issue]);
    } else {
      state.general.push(issue);
    }
    if (blocking) {
      const invariant = classifyInvariant(issue.code);
      if (invariant) state.invariants[invariant] = "fail";
    }
  }
  state.hasBlockingErrors = state.errorCount > 0;
  return state;
}

/** Returns a new node array with `data.hasError` reflecting the per-node lint map. */
export function applyLintToNodes(
  nodes: readonly EditorNode[],
  byNode: ReadonlyMap<string, LoopValidationIssue[]>
): EditorNode[] {
  return nodes.map(node => {
    const hasError = byNode.has(node.id);
    if (hasError === node.data.hasError) return node;
    return { ...node, data: { ...node.data, hasError } };
  });
}
