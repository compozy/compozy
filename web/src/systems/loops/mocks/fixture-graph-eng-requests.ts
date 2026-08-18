import type { LoopRequest } from "../types";

export const GRAPH_ENG_RUN_ID = "looprun_release_train";
export const GRAPH_ENG_FORK_RUN_ID = "looprun_release_train_fork";
export const GRAPH_ENG_TERMINAL_RUN_ID = "looprun_release_train_done";

const LOOP_NAME = "release-train";
const OPENED_AT = "2026-08-17T09:00:00Z";

const ANSWER_SCHEMA = {
  type: "object",
  required: ["regions", "canary"],
  properties: {
    regions: { type: "array", items: { type: "string" }, description: "Regions to ship first." },
    canary: { type: "boolean", description: "Run a canary before the full rollout." },
  },
} as const;

const EDIT_SCHEMA = {
  type: "object",
  required: ["tag"],
  properties: {
    tag: { type: "string" },
    dry_run: { type: "boolean" },
  },
} as const;

const RESPOND_SCHEMA = {
  type: "object",
  required: ["migration_url"],
  properties: { migration_url: { type: "string" } },
} as const;

function request(
  overrides: Partial<LoopRequest> & Pick<LoopRequest, "node_id" | "kind">
): LoopRequest {
  return {
    loop_run_id: GRAPH_ENG_RUN_ID,
    loop_name: LOOP_NAME,
    generation: 3,
    item_index: 0,
    state: "pending",
    prompt: "",
    context: {},
    decisions: ["respond"],
    agents: "deny",
    opened_at: OPENED_AT,
    ...overrides,
  };
}

export const pendingAskRequest = request({
  node_id: "confirm-rollout",
  kind: "ask",
  prompt: "Which regions ship first?",
  context: { train: "release-train", services: ["api", "web", "worker"] },
  expect: ANSWER_SCHEMA,
  decisions: ["respond"],
  agents: "allow",
  expires_at: "2026-08-18T09:00:00Z",
});

export const pendingEnumAskRequest = request({
  node_id: "choose-rollout",
  kind: "ask",
  prompt: "Approve this rollout?",
  context: { train: "release-train" },
  expect: {
    type: "object",
    required: ["decision"],
    properties: {
      decision: { enum: ["approve", "discard"] },
    },
  },
  decisions: ["respond"],
  agents: "allow",
  expires_at: "2026-08-18T09:00:00Z",
});

export const pendingReviewRequest = request({
  node_id: "apply-migration",
  kind: "review",
  prompt: "apply-migration proposes a migrate",
  context: { service: "billing", window: "2026-08-17 22:00 UTC" },
  proposed_preview: { tag: "high-billing", dry_run: true, notes: "Rollout for billing" },
  edit_schema: EDIT_SCHEMA,
  respond_schema: RESPOND_SCHEMA,
  decisions: ["approve", "edit", "reject", "respond"],
  agents: "deny",
  expires_at: "2026-08-17T22:00:00Z",
});

export const nearExpiryAskRequest = request({
  node_id: "confirm-rollout",
  kind: "ask",
  prompt: "Which regions ship first?",
  context: { train: "release-train" },
  expect: ANSWER_SCHEMA,
  expires_at: "2026-08-17T09:04:00Z",
});

export const answeredAskRequest = request({
  node_id: "confirm-rollout",
  kind: "ask",
  state: "answered",
  prompt: "Which regions ship first?",
  context: { train: "release-train" },
  expect: ANSWER_SCHEMA,
  answered_decision: "respond",
  answered_at: "2026-08-17T09:12:00Z",
  resolved_at: "2026-08-17T09:12:00Z",
  actor_kind: "operator",
  actor_id: "pedro",
});

export const expiredAskRequest = request({
  node_id: "confirm-rollout",
  kind: "ask",
  state: "expired",
  prompt: "Which regions ship first?",
  context: { train: "release-train" },
  expect: ANSWER_SCHEMA,
  expires_at: "2026-08-17T09:02:00Z",
  resolved_at: "2026-08-17T09:02:00Z",
});

export const canceledReviewRequest = request({
  node_id: "apply-migration",
  kind: "review",
  state: "canceled",
  prompt: "apply-migration proposes a migrate",
  proposed_preview: { tag: "high-billing" },
  decisions: ["approve", "reject"],
  resolved_at: "2026-08-17T09:30:00Z",
  actor_kind: "operator",
  actor_id: "pedro",
});

export const redactedContextRequest = request({
  node_id: "apply-migration",
  kind: "review",
  prompt: "apply-migration proposes a migrate",
  context: {
    service: "billing",
    credentials: "••• redacted",
    changelog: "The first 2 KB of the changelog…",
    truncated: true,
  },
  proposed_preview: { tag: "high-billing", token: "••• redacted" },
  decisions: ["approve", "reject"],
  expires_at: "2026-08-17T22:00:00Z",
});

export const laneAskRequests: LoopRequest[] = ["api", "web", "worker"].map((lane, index) =>
  request({
    node_id: "apply-migration",
    kind: "review",
    item_index: index,
    prompt: `apply-migration proposes a migrate for ${lane}`,
    context: { service: lane },
    proposed_preview: { tag: `high-${lane}` },
    decisions: ["approve", "reject"],
    expires_at: "2026-08-17T22:00:00Z",
  })
);

export const graphEngPendingRequests: LoopRequest[] = [pendingAskRequest, pendingReviewRequest];

export const graphEngResolvedRequests: LoopRequest[] = [
  answeredAskRequest,
  expiredAskRequest,
  canceledReviewRequest,
];

export const graphEngRequestsByNode = new Map<string, LoopRequest>(
  [...graphEngPendingRequests, pendingEnumAskRequest, ...graphEngResolvedRequests].map(entry => [
    `${entry.loop_run_id}:${entry.generation}:${entry.node_id}:${entry.item_index}`,
    entry,
  ])
);
