import type { RawLoopNode } from "./codec";
import { getAtPath, type NodeFieldEdit } from "./loop-editor-draft";
import { str } from "./loop-node-fields";
import {
  LOOP_REVIEW_DECISIONS,
  type LoopReviewDecision,
  type ReviewFieldSpec,
} from "./loop-node-schema-types";

function isReviewDecision(value: string): value is LoopReviewDecision {
  return (LOOP_REVIEW_DECISIONS as readonly string[]).includes(value);
}

export function readReview(raw: RawLoopNode, targets: string[] = []): ReviewFieldSpec {
  const value = getAtPath(raw, ["review"]);
  const record =
    typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
  const decisions = Array.isArray(record?.decisions)
    ? record.decisions.filter((entry): entry is string => typeof entry === "string")
    : [];
  const responders =
    typeof record?.responders === "object" && record.responders !== null
      ? (record.responders as Record<string, unknown>)
      : null;
  const onReject =
    typeof record?.on_reject === "object" && record.on_reject !== null
      ? (record.on_reject as Record<string, unknown>)
      : null;
  const expires =
    typeof record?.expires === "object" && record.expires !== null
      ? (record.expires as Record<string, unknown>)
      : null;
  return {
    type: "review",
    key: "review",
    label: "Review before running",
    basePath: ["review"],
    enabled: record !== null,
    decisions: decisions.filter(isReviewDecision),
    when: str(record?.when),
    prompt: str(record?.prompt),
    agentsAllowed: str(responders?.agents) === "allow",
    onRejectRoute: str(onReject?.route),
    expiresAfter: str(expires?.after),
    targets,
    hint: "Parks the run before this node executes so a human decides on the exact arguments. Leaving the decisions empty means approve or reject.",
  };
}

export interface ReviewDraft {
  enabled: boolean;
  decisions: LoopReviewDecision[];
  when: string;
  prompt: string;
  agentsAllowed: boolean;
  onRejectRoute: string;
  expiresAfter: string;
}

export function reviewEdits(draft: ReviewDraft): NodeFieldEdit[] {
  if (!draft.enabled) return [{ path: ["review"], value: undefined }];
  const value: Record<string, unknown> = {};
  if (draft.decisions.length > 0) value.decisions = [...draft.decisions];
  if (draft.when.trim() !== "") value.when = draft.when.trim();
  if (draft.prompt.trim() !== "") value.prompt = draft.prompt.trim();

  if (draft.agentsAllowed) value.responders = { agents: "allow" };
  if (draft.onRejectRoute.trim() !== "") value.on_reject = { route: draft.onRejectRoute.trim() };
  if (draft.expiresAfter.trim() !== "") value.expires = { after: draft.expiresAfter.trim() };
  return [{ path: ["review"], value }];
}
