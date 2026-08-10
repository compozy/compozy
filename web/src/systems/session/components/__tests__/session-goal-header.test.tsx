import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";
import { sessionStore } from "@/systems/session/stores/session-store";
import type { SessionGoalCommandResult, SessionGoalSnapshot } from "@/systems/session/types";

vi.mock("@tanstack/react-router", async importOriginal => {
  const actual = await importOriginal<typeof import("@tanstack/react-router")>();
  return {
    ...actual,
    Link: ({ to, params, hash, children, ...props }: Record<string, unknown>) => (
      <a
        href={typeof to === "string" ? to : "#"}
        data-params={JSON.stringify(params)}
        data-hash={hash}
        {...(props as Record<string, unknown>)}
      >
        {children as React.ReactNode}
      </a>
    ),
  };
});

const { SessionGoalHeaderContainer } = await import("../goal/session-goal-header-container");
const { SessionGoalHeadAction } = await import("../goal/goal-head-action");
const { useSessionGoalHeader } = await import("../../hooks/use-session-goal-header");

const WORKSPACE_ID = "ws_1";
const SESSION_ID = "session_1";

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: SESSION_ID,
    cause: null,
    context: {
      nudge_ratio: 0.8,
      ratio: 0.5,
      reported_at: "2026-07-10T12:00:00Z",
      size: 200_000,
      state: "known",
      used: 100_000,
    },
    contract_summary: "Ship after verification.",
    last_verdict: null,
    live: true,
    node_id: "goal",
    objective: "Ship the Goal surface.",
    origin_session_id: SESSION_ID,
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 7,
    ...overrides,
  };
}

class FakeEventSource {
  static latest: FakeEventSource | null = null;
  readonly listeners = new Map<string, Set<EventListenerOrEventListenerObject>>();
  onmessage: ((event: MessageEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  close = vi.fn();

  constructor(readonly url: string) {
    FakeEventSource.latest = this;
  }

  addEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    const listeners = this.listeners.get(type) ?? new Set<EventListenerOrEventListenerObject>();
    listeners.add(listener);
    this.listeners.set(type, listeners);
  }

  removeEventListener(type: string, listener: EventListenerOrEventListenerObject) {
    this.listeners.get(type)?.delete(listener);
  }

  dispatch(kind: string, payload: unknown, seq: number) {
    const frame = {
      id: `event_${seq}`,
      seq,
      kind,
      loop_run_id: "run_1",
      workspace_id: WORKSPACE_ID,
      at: "2026-07-10T12:01:00Z",
      payload,
    };
    const event = new MessageEvent(kind, { data: JSON.stringify(frame) });
    for (const listener of this.listeners.get(kind) ?? []) {
      if (typeof listener === "function") listener(event);
      else listener.handleEvent(event);
    }
  }
}

function path(input: RequestInfo | URL): string {
  if (typeof input === "string") return new URL(input, "http://localhost").pathname;
  if (input instanceof URL) return input.pathname;
  return new URL(input.url, "http://localhost").pathname;
}

function requestBody(
  input: RequestInfo | URL,
  init?: RequestInit
): Promise<Record<string, unknown>> {
  if (input instanceof Request) return input.clone().json() as Promise<Record<string, unknown>>;
  if (typeof init?.body === "string") {
    return Promise.resolve(JSON.parse(init.body) as Record<string, unknown>);
  }
  return Promise.resolve({});
}

function latestPromptText(body: Record<string, unknown>): string {
  if (!Array.isArray(body.messages)) return "";
  const message = body.messages.at(-1);
  if (typeof message !== "object" || message === null || !("parts" in message)) return "";
  if (!Array.isArray(message.parts)) return "";
  const parts = message.parts as unknown[];
  return parts
    .filter(
      (part: unknown): part is { text: string; type: "text" } =>
        typeof part === "object" &&
        part !== null &&
        "type" in part &&
        part.type === "text" &&
        "text" in part &&
        typeof part.text === "string"
    )
    .map(part => part.text)
    .join("\n");
}

// Mirrors the production split: the strip (container) owns stream
// reconciliation; the window-head action reads the shared query cache.
function HeadActionHarness() {
  const goal = useSessionGoalHeader(WORKSPACE_ID, SESSION_ID, { stream: false });
  return (
    <SessionGoalHeadAction
      snapshot={goal.snapshot}
      pendingAction={goal.pendingAction}
      onPause={goal.onPause}
      onResume={goal.onResume}
      onApprove={goal.onApprove}
      onClear={goal.onClear}
    />
  );
}

function renderHeader(setComposerText = vi.fn()) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const result = render(
    <QueryClientProvider client={queryClient}>
      <HeadActionHarness />
      <SessionGoalHeaderContainer
        onPrefillComposer={setComposerText}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
      />
    </QueryClientProvider>
  );
  return { ...result, queryClient, setComposerText };
}

describe("SessionGoalHeader", () => {
  let visibleGoal: SessionGoalSnapshot | null;
  let goalReads: number;
  let commands: string[];

  beforeEach(() => {
    visibleGoal = snapshot();
    goalReads = 0;
    commands = [];
    FakeEventSource.latest = null;
    resetGatewayStreamAuth();
    sessionStore.trigger.sessionInteractionRemoved({ sessionId: SESSION_ID });
    vi.stubGlobal("EventSource", FakeEventSource);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const pathname = path(input);
        if (pathname.endsWith("/api/status")) {
          return Response.json({}, { headers: { "X-Compozy-Gateway-Tier": "local" } });
        }
        if (pathname.endsWith(`/sessions/${SESSION_ID}/goal`)) {
          goalReads += 1;
          return Response.json({ goal: visibleGoal });
        }
        if (pathname.endsWith(`/sessions/${SESSION_ID}/prompt`)) {
          const body = await requestBody(input, init);
          const message = latestPromptText(body);
          commands.push(message);
          if (message === "/goal pause") {
            visibleGoal = snapshot({ status: "paused", run_status: "paused" });
            return Response.json({
              prompt: {
                goal: {
                  outcome: "paused",
                  reason_code: null,
                  replaced_run_id: null,
                  snapshot: visibleGoal,
                } satisfies SessionGoalCommandResult,
                idempotency_key: body.idempotency_key,
                message_id: body.message_id,
                replayed: false,
                status: "accepted",
              },
            });
          }
          if (message === "/goal clear") {
            visibleGoal = null;
            return Response.json({
              prompt: {
                goal: {
                  outcome: "cleared",
                  reason_code: null,
                  replaced_run_id: null,
                  snapshot: null,
                } satisfies SessionGoalCommandResult,
                idempotency_key: body.idempotency_key,
                message_id: body.message_id,
                replayed: false,
                status: "accepted",
              },
            });
          }
        }
        throw new Error(`Unhandled Goal header request: ${pathname}`);
      })
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
    resetGatewayStreamAuth();
  });

  it("Should merge live turns, invalidate only on status, and pause from the window head", async () => {
    renderHeader();
    expect(await screen.findByTestId("session-goal-header")).toBeInTheDocument();
    expect(goalReads).toBe(1);
    await waitFor(() => expect(FakeEventSource.latest?.url).toContain("/loop-runs/run_1/events"));
    await waitFor(() =>
      expect(FakeEventSource.latest?.listeners.has("goal_turn_started")).toBe(true)
    );

    act(() => {
      FakeEventSource.latest?.dispatch("goal_turn_started", { turn: 8 }, 1);
    });
    // The strip facts carry the merged turn count as mono text.
    await waitFor(() =>
      expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent("turn 8/20 · ctx 50%")
    );
    expect(goalReads).toBe(1);

    act(() => {
      FakeEventSource.latest?.dispatch("goal_status_changed", { from: "active", to: "paused" }, 2);
    });
    await waitFor(() => expect(goalReads).toBe(2));

    // The one gated head action pauses the goal; the strip reflects the change
    // and the head action flips to Resume (never two actions at once).
    fireEvent.click(screen.getByTestId("goal-head-pause"));
    await waitFor(() => expect(commands).toContain("/goal pause"));
    await waitFor(() =>
      expect(screen.getByRole("region", { name: "Goal status" })).toHaveAttribute(
        "data-goal-status",
        "paused"
      )
    );
    expect(screen.getByTestId("goal-head-resume")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-head-pause")).not.toBeInTheDocument();
  });

  it("Should clear only a terminal snapshot from the window head", async () => {
    visibleGoal = snapshot({ live: false, status: "complete", run_status: "done" });
    renderHeader();

    const clear = await screen.findByTestId("goal-head-clear");
    expect(screen.queryByTestId("goal-head-pause")).not.toBeInTheDocument();
    fireEvent.click(clear);

    await waitFor(() => expect(commands).toContain("/goal clear"));
    await waitFor(() =>
      expect(screen.queryByTestId("session-goal-header")).not.toBeInTheDocument()
    );
  });

  it("Should preserve the current stale-replacement snapshot and expose its draft affordance", async () => {
    const current = snapshot({ objective: "Current objective" });
    visibleGoal = current;
    sessionStore.trigger.goalCommandReported({
      sessionId: SESSION_ID,
      result: {
        outcome: "error",
        reason_code: "goal_replace_stale",
        replaced_run_id: null,
        snapshot: current,
      },
      command: "/goal Requested replacement",
    });
    renderHeader(vi.fn());

    // The prefill affordance lives in the strip body — open the strip first.
    fireEvent.click(await screen.findByTestId("goal-strip-line"));
    expect(screen.getByRole("button", { name: "Draft replacement" })).toBeInTheDocument();
    expect(screen.getByRole("region", { name: "Goal status" })).toHaveTextContent(
      "Current objective"
    );
  });
});
