import type { SubagentStatus, SubagentView } from "../types";

// Board fixtures (DESIGN-NOTES: values are fixtures; runtime truth owns them).
// Timestamps are relative to story load so live rows tick from a known offset.
export const NO_RUNTIME = { agent: "", provider: "", model: "", reasoning_effort: "", speed: "" };

const LOADED_AT = Date.now();
export const secondsAgo = (seconds: number) => new Date(LOADED_AT - seconds * 1_000).toISOString();

const LIVE: ReadonlySet<SubagentStatus> = new Set(["queued", "running", "waiting"]);

export interface FixtureOptions extends Partial<SubagentView> {
  /** Seconds since the daemon started the child. */
  elapsed?: number;
}

export function subagentFixture({ elapsed = 7, ...overrides }: FixtureOptions = {}): SubagentView {
  const status = overrides.status ?? "running";
  const live = LIVE.has(status);
  return {
    id: overrides.id ?? `sub-${overrides.title ?? "fixture"}`,
    parent_session_id: "sess-7f3a2c11d09e4b58",
    role: "general",
    child_session_id: "sess-b41d77e05a3c9f12",
    origin: "delegated",
    title: "Audit payment webhooks for retry safety",
    status,
    work_state: live ? "working" : "result_available",
    progress: "",
    result_preview: "",
    error: null,
    runtime: {
      ...NO_RUNTIME,
      agent: "claude",
      provider: "claude",
      model: "Opus 5.5",
      reasoning_effort: "high",
    },
    depth: 1,
    started_at: secondsAgo(live ? elapsed : elapsed + 30),
    settled_at: live ? null : secondsAgo(30),
    created_at: secondsAgo(elapsed + 31),
    updated_at: secondsAgo(live ? 0 : 30),
    ...overrides,
  };
}

const codex = {
  agent: "reviewer",
  provider: "codex",
  model: "gpt-5.6-sol",
  reasoning_effort: "medium",
};

export const cardStates = {
  queued: subagentFixture({
    id: "queued",
    status: "queued",
    title: "Migrate orders.refund_reason to an enum",
    elapsed: 3,
  }),
  running: subagentFixture({ id: "running", elapsed: 7 }),
  runningProgress: subagentFixture({
    id: "running-progress",
    elapsed: 725,
    progress: 'Searching for "idempotency_key" in internal/payments',
  }),
  waiting: subagentFixture({
    id: "waiting",
    status: "waiting",
    title: "Review PR #812 for N+1 queries",
    runtime: { ...NO_RUNTIME, ...codex, speed: "fast" },
    elapsed: 98,
  }),
  completed: subagentFixture({
    id: "completed",
    status: "completed",
    title: "Draft release notes for v2",
    result_preview: "Drafted notes: 6 user-facing changes, 2 fixes, 1 migration step.",
    elapsed: 52,
  }),
  failed: subagentFixture({
    id: "failed",
    status: "failed",
    title: "Run the checkout e2e suite",
    runtime: { ...NO_RUNTIME, provider: "claude", model: "Sonnet 5.5", reasoning_effort: "medium" },
    error:
      '3 of 41 specs failed in checkout/refund.spec.ts: "refund shows reason" timed out after 30s waiting for [data-testid=refund-reason].',
    elapsed: 252,
  }),
  canceled: subagentFixture({
    id: "canceled",
    status: "canceled",
    title: "Profile the cart endpoint under load",
    runtime: { ...NO_RUNTIME, ...codex },
    elapsed: 61,
  }),
  interrupted: subagentFixture({
    id: "interrupted",
    status: "interrupted",
    title: "Backfill refund_reason for 2025 orders",
    progress: "Updated 18,400 of 52,000 rows",
    elapsed: 3_780,
  }),
  unknownProvider: subagentFixture({
    id: "unknown-provider",
    title: "Summarize open support tickets about refunds",
    runtime: { ...NO_RUNTIME, provider: "acme-agent" },
    progress: "Reading tickets/2026-10",
    elapsed: 44,
  }),
  stale: subagentFixture({
    id: "stale",
    progress: "Reading internal/payments/webhook.go",
    elapsed: 161,
    updated_at: secondsAgo(0),
  }),
  longTitle: subagentFixture({
    id: "long-title",
    title: "Investigate why refund webhooks retry twice when Stripe returns a 409 too",
    progress: "Reading internal/payments/stripe_client.go",
    elapsed: 19,
  }),
} satisfies Record<string, SubagentView>;

export const nativeRunning = subagentFixture({
  id: "native",
  origin: "provider_native",
  child_session_id: null,
  title: "Explore how refunds are persisted",
  progress: 'Grep "RefundReason" in internal/',
  elapsed: 23,
});

export const nativeSettled = subagentFixture({
  id: "native-settled",
  origin: "provider_native",
  child_session_id: null,
  status: "completed",
  title: "Explore how refunds are persisted",
  result_preview: "Refunds live in refunds.reason (text); no enum yet, 3 writers.",
  elapsed: 48,
});

export const settledResultLong = subagentFixture({
  id: "settled-long",
  status: "completed",
  title: "Review PR #812 for N+1 queries",
  runtime: { ...NO_RUNTIME, ...codex, speed: "fast" },
  elapsed: 305,
  result_preview:
    'Found 2 N+1 queries. OrderSummary.items loads each line\'s product in a loop (orders/summary.go:88), and RefundList renders the customer for every row without a preload. Suggested Preload("Items.Product") and a joined customer query; both are covered by existing tests, and no migration is needed for either change, so the PR can merge after the two preloads land.',
});

/** The ten-subagent fan-out from the navigation board. */
export const roster: SubagentView[] = [
  { ...cardStates.failed, created_at: secondsAgo(900) },
  subagentFixture({
    id: "copy-check",
    status: "failed",
    title: "Check refund copy against COPY.md",
    error: "COPY.md has no refund section.",
    created_at: secondsAgo(880),
  }),
  { ...cardStates.waiting, created_at: secondsAgo(400) },
  {
    ...cardStates.running,
    progress: "Reading internal/payments/webhook.go",
    started_at: secondsAgo(161),
    created_at: secondsAgo(300),
  },
  { ...cardStates.canceled, status: "running", settled_at: null, created_at: secondsAgo(200) },
  { ...cardStates.completed, created_at: secondsAgo(600) },
  { ...nativeSettled, created_at: secondsAgo(590) },
  subagentFixture({
    id: "callers",
    status: "completed",
    title: "List callers of OrderSummary",
    elapsed: 33,
    created_at: secondsAgo(580),
  }),
  { ...cardStates.unknownProvider, status: "completed", created_at: secondsAgo(570) },
  { ...cardStates.interrupted, status: "canceled", created_at: secondsAgo(560) },
];

export function migrationReviews(count: number): SubagentView[] {
  return Array.from({ length: count }, (_, index) =>
    subagentFixture({
      id: `migration-${index}`,
      status: "completed",
      title: `Review migrations/${String(101 - index).padStart(4, "0")}_refund_reason.sql`,
      elapsed: 27 + ((index * 7) % 25),
      created_at: secondsAgo(1_000 + index),
    })
  );
}
