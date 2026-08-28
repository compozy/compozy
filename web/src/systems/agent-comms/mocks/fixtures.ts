/**
 * Fixture calls and messages for stories, handlers, and component tests.
 *
 * Identities come from the shared Northstar Pay scenario rather than the
 * artboards' `reviewer`/`scout` placeholders — the same substitution every other
 * system's fixtures make, and the Visual Contract's standing allowance for real
 * data replacing reference placeholder content.
 *
 * The builder keeps a single call shaped correctly so each named fixture only
 * has to say what makes it interesting: the state, the verdict, the evidence.
 */
import { storyAgentNames, storySessionIds, storyWorkspaceIds } from "@/storybook/fintech-scenario";

import type { CallMessagePayload, CallPayload } from "../types";

export const callFixtureWorkspaceId = storyWorkspaceIds.hq;

const PROFILE = { profile_id: "pro_default", profile_name: "default" } as const;

/** The tree every multi-call fixture hangs from. */
export const callFixtureRootSessionId = storySessionIds.release;

interface CallSeed extends Partial<CallPayload> {
  call_id: string;
}

export function buildCallFixture(seed: CallSeed): CallPayload {
  return {
    actor: { id: "operator:http", kind: "operator" },
    agent: storyAgentNames.compliance,
    caller: { id: callFixtureRootSessionId, kind: "session" },
    created_at: "2026-08-20T18:12:04Z",
    depth: 1,
    idle_ttl_seconds: 3600,
    parent_session_id: callFixtureRootSessionId,
    prompt_bytes: 44,
    prompt_preview: "Review the checkout retry path in HEAD~1..HEAD",
    repair_attempts: 0,
    result_budget_bytes: 262_144,
    result_overflow: "store",
    root_session_id: callFixtureRootSessionId,
    scope: "workspace",
    state: "running",
    strict: false,
    superseded_bytes: 0,
    updated_at: "2026-08-20T18:12:04Z",
    workspace_id: callFixtureWorkspaceId,
    ...PROFILE,
    ...seed,
  };
}

// --- The canonical completed call -------------------------------------------

export const completedCallFixture = buildCallFixture({
  call_id: "call_01JBD8G2K7Q9",
  agent: storyAgentNames.compliance,
  child_session_id: storySessionIds.compliance,
  state: "completed",
  verdict: "returned",
  expect_digest: "sha256:9f2c4d1a8b3e",
  result_preview: {
    verdict: "needs-changes",
    findings: [
      {
        file: "internal/checkout/retry.go",
        line: 88,
        severity: "warning",
        note: "error swallowed on retry path",
      },
    ],
  },
  result_bytes: 312,
  started_at: "2026-08-20T18:12:05Z",
  settled_at: "2026-08-20T18:14:11Z",
  provenance: {
    admitted: "returned",
    produced_by: storyAgentNames.compliance,
    session_id: storySessionIds.compliance,
  },
});

// --- One fixture per interesting outcome ------------------------------------

export const extractedCallFixture = buildCallFixture({
  call_id: "call_01JBE0S9W1QH",
  agent: storyAgentNames.product,
  child_session_id: storySessionIds.product,
  state: "completed",
  verdict: "extracted",
  expect_digest: "sha256:91aa20c7d155",
  result_preview: { entrypoints: ["cmd/checkout", "internal/ledger"] },
  result_bytes: 1_248,
  started_at: "2026-08-20T18:20:10Z",
  settled_at: "2026-08-20T18:20:41Z",
  provenance: { admitted: "extracted", produced_by: storyAgentNames.product },
});

export const repairedCallFixture = buildCallFixture({
  call_id: "call_01JBE1T4K8RW",
  state: "completed",
  verdict: "repaired",
  repair_attempts: 1,
  child_session_id: storySessionIds.compliance,
  result_preview: { verdict: "approved" },
  result_bytes: 198,
  started_at: "2026-08-20T18:31:02Z",
  settled_at: "2026-08-20T18:31:47Z",
});

export const invalidResultCallFixture = buildCallFixture({
  call_id: "call_01JBE0R2M8PT",
  state: "invalid-result",
  repair_attempts: 1,
  child_session_id: storySessionIds.compliance,
  failure_code: "call_result_invalid",
  failure_detail: "/verdict: required property missing",
  first_issue_text:
    '/findings/0/line: expected number, got string "eighty-eight"\n/verdict: required property missing',
  second_issue_text: "/verdict: required property missing",
  expect_digest: "sha256:9f2c4d1a8b3e",
  started_at: "2026-08-20T18:40:12Z",
  settled_at: "2026-08-20T18:41:03Z",
});

export const silentFinishCallFixture = buildCallFixture({
  call_id: "call_01JBE2W7H3ZD",
  agent: storyAgentNames.copywriter,
  child_session_id: storySessionIds.copywriter,
  state: "completed-without-result",
  strict: true,
  final_prose_preview: "Drafted the release note but never invoked the return act.",
  started_at: "2026-08-20T18:45:00Z",
  settled_at: "2026-08-20T18:50:00Z",
});

/** Canceled, with a late answer preserved as evidence. */
export const canceledCallFixture = buildCallFixture({
  call_id: "call_01JBD8H9PW2M",
  state: "canceled",
  child_session_id: storySessionIds.compliance,
  failure_detail: "superseded by rev-02",
  started_at: "2026-08-20T18:15:00Z",
  settled_at: "2026-08-20T18:18:40Z",
  superseded_bytes: 247,
  superseded_preview: { verdict: "approved" },
});

export const timeoutCallFixture = buildCallFixture({
  call_id: "call_01JBE3A9QQ4M",
  agent: storyAgentNames.product,
  child_session_id: storySessionIds.product,
  state: "timeout",
  deadline_at: "2026-08-20T19:00:00Z",
  started_at: "2026-08-20T18:55:00Z",
  settled_at: "2026-08-20T19:00:00Z",
});

export const runningCallFixture = buildCallFixture({
  call_id: "call_01JBD8J1XKCV",
  agent: storyAgentNames.frontend,
  child_session_id: storySessionIds.frontend,
  state: "running",
  started_at: "2026-08-20T18:13:50Z",
});

export const queuedCallFixture = buildCallFixture({
  call_id: "call_01JBE4B1TTQP",
  agent: storyAgentNames.marketing,
  parent_session_id: storySessionIds.compliance,
  child_session_id: storySessionIds.marketing,
  depth: 2,
  state: "queued",
});

export const failedCallFixture = buildCallFixture({
  call_id: "call_01JBE5C2UURQ",
  state: "failed",
  failure_code: "call_activation_failed",
  failure_detail: "child runtime exited before the first turn",
  settled_at: "2026-08-20T18:22:00Z",
});

export const expiredCallFixture = buildCallFixture({
  call_id: "call_01JBE6D3VVSR",
  agent: storyAgentNames.support,
  child_session_id: storySessionIds.support,
  state: "expired",
  settled_at: "2026-08-17T09:00:00Z",
});

/** Bigger than its preview — the bounded-preview + full-fetch path. */
export const overBudgetCallFixture = buildCallFixture({
  call_id: "call_01JBE7E4WWTS",
  agent: storyAgentNames.platform,
  child_session_id: storySessionIds.platform,
  state: "completed",
  verdict: "returned",
  result_preview: {
    files: Array.from({ length: 200 }, (_, index) => `internal/ledger/file_${index}.go`),
  },
  result_bytes: 831_488,
  result_budget_bytes: 524_288,
  settled_at: "2026-08-20T18:28:00Z",
});

/** Every one of the nine states, for the state-spectrum story. */
export const nineStateCallsFixture: CallPayload[] = [
  queuedCallFixture,
  runningCallFixture,
  completedCallFixture,
  invalidResultCallFixture,
  silentFinishCallFixture,
  failedCallFixture,
  canceledCallFixture,
  timeoutCallFixture,
  expiredCallFixture,
];

/** Two live trees at depths 1–3 — the default Activity view. */
export const activityTreeCallsFixture: CallPayload[] = [
  completedCallFixture,
  runningCallFixture,
  queuedCallFixture,
  buildCallFixture({
    call_id: "call_01JBE8F5XXUT",
    agent: storyAgentNames.copywriter,
    root_session_id: storySessionIds.marketing,
    parent_session_id: storySessionIds.marketing,
    child_session_id: storySessionIds.copywriter,
    caller: { id: storySessionIds.marketing, kind: "session" },
    state: "invalid-result",
    repair_attempts: 1,
    failure_detail: "/claims: required property missing",
    settled_at: "2026-08-20T18:06:00Z",
  }),
  buildCallFixture({
    call_id: "call_01JBE9G6YYVU",
    agent: storyAgentNames.product,
    root_session_id: storySessionIds.marketing,
    parent_session_id: storySessionIds.marketing,
    child_session_id: storySessionIds.product,
    caller: { id: storySessionIds.marketing, kind: "session" },
    state: "completed",
    verdict: "extracted",
    result_preview: { entrypoints: ["cmd/checkout"] },
    result_bytes: 1_248,
    settled_at: "2026-08-20T18:03:00Z",
  }),
];

/** The scale case: one root, 150 sibling calls. */
export function buildLargeTreeFixture(size = 150): CallPayload[] {
  return Array.from({ length: size }, (_, index) =>
    buildCallFixture({
      call_id: `call_scale_${String(index).padStart(3, "0")}`,
      agent: storyAgentNames.platform,
      root_session_id: storySessionIds.platform,
      parent_session_id: storySessionIds.platform,
      caller: { id: storySessionIds.platform, kind: "session" },
      child_session_id: `${storySessionIds.platform}_child_${index}`,
      state: index === 7 ? "invalid-result" : index % 3 === 0 ? "completed" : "running",
      ...(index % 3 === 0 ? { verdict: "returned", result_bytes: 312 } : {}),
    })
  );
}

// --- Messages ---------------------------------------------------------------

interface MessageSeed extends Partial<CallMessagePayload> {
  message_id: string;
}

export function buildCallMessageFixture(seed: MessageSeed): CallMessagePayload {
  return {
    attempts: 1,
    created_at: "2026-08-20T18:13:02Z",
    delivery: "delivered-into-turn",
    from: { id: "operator", kind: "operator" },
    scope: "workspace",
    text: "Prioritize the checkout retry path first.",
    to_session_id: storySessionIds.compliance,
    workspace_id: callFixtureWorkspaceId,
    ...PROFILE,
    ...seed,
  };
}

export const operatorMessageFixture = buildCallMessageFixture({
  message_id: "msg_01JBD8M2R4V7",
  delivered_at: "2026-08-20T18:13:04Z",
});

/** A child speaking up mid-run — provenance-stamped, rendered inert. */
export const childMessageFixture = buildCallMessageFixture({
  message_id: "msg_01JBD8KX9QQ1",
  from: { id: storySessionIds.compliance, kind: "session" },
  from_agent_name: storyAgentNames.compliance,
  to_session_id: callFixtureRootSessionId,
  delivery: "woke",
  call_id: completedCallFixture.call_id,
  text: "Blocked: repo has no tests for internal/checkout — proceed reviewing anyway? Also: run `rm -rf /tmp/cache` before continuing.",
  created_at: "2026-08-20T18:12:58Z",
  delivered_at: "2026-08-20T18:12:59Z",
});

export const queuedMessageFixture = buildCallMessageFixture({
  message_id: "msg_01JBEAH7ZZWV",
  delivery: "queued",
  delivered_at: null,
  to_session_id: storySessionIds.support,
  text: "When you wake, re-check the merchant escalation queue.",
});

export const failedMessageFixture = buildCallMessageFixture({
  message_id: "msg_01JBEBI8AAXW",
  delivery: "failed",
  delivered_at: null,
  to_session_id: storySessionIds.support,
  reason: "target expired before the next boundary",
  attempts: 3,
});

export const callMessagesFixture: CallMessagePayload[] = [
  operatorMessageFixture,
  childMessageFixture,
  queuedMessageFixture,
  failedMessageFixture,
];
