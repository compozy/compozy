import type { EditorNode } from "./codec";
import type { LoopValidationIssue, ValidateLoopResult } from "../types";

/**
 * Maps the shared Go linter's verdict (`POST /loops/:name/validate` and the publish
 * 422, ADR-023) onto the editor: the four canonical invariant chips (ADR-015) plus a
 * reference chip (ADR-020), the per-node error map the canvas badges and inspector
 * highlight consume, and the blocking-error flag that gates Publish. The GUI never
 * recomputes invariants — it only surfaces what the daemon returned.
 */

export type LoopInvariantKey =
  | "acyclicity"
  | "reachability"
  | "termination"
  | "fan_out"
  | "references";

export type LoopInvariantStatus = "pass" | "fail";

/**
 * The four canonical structural invariants (ADR-015: acyclicity, reachability,
 * termination, fan-out bounds) PLUS a fifth References chip. Reference validation is a
 * real, first-class linter output (ADR-020) that fails Publish, so it earns a chip even
 * though the spec text says "the 4 invariant chips" — the four canonical keys are the
 * ones tests assert; References is the honest fifth surface, not decoration.
 */
export const LOOP_INVARIANTS: { key: LoopInvariantKey; label: string }[] = [
  { key: "acyclicity", label: "Acyclicity" },
  { key: "reachability", label: "Reachability" },
  { key: "termination", label: "Termination" },
  { key: "references", label: "References" },
  { key: "fan_out", label: "Fan-out bounds" },
];

// Deterministic-code → invariant classification. Codes are matched by substring so a
// daemon that appends new codes in a known family still lights the right chip; an
// unrecognized error code stays in the issue list and blocks Publish without flipping
// a named chip (truthful UI: we never claim an invariant we can't attribute).
// `references` is matched before `fan_out` on purpose: `item_outside_fanout` is a
// reference-scope error (ADR-020, `item`/`index` used outside a fan-out branch body), but
// its substring `fanout` would otherwise misattribute it to the Fan-out bounds chip.
const CODE_KEYWORDS: { key: LoopInvariantKey; keywords: string[] }[] = [
  { key: "acyclicity", keywords: ["cycle", "acyclic"] },
  { key: "reachability", keywords: ["unreachable", "reachab"] },
  { key: "termination", keywords: ["terminat", "no_terminal", "nonterminating"] },
  {
    key: "references",
    keywords: ["reference", "unresolvable", "condition_not_bool", "item_outside_fanout", "node_id"],
  },
  { key: "fan_out", keywords: ["fan_out", "fanout"] },
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
    },
    hasBlockingErrors: false,
    errorCount: 0,
    validated: false,
  };
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
