import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { resetGatewayStreamAuth } from "@/lib/gateway-stream-auth";
import { sessionStore } from "@/systems/session/stores/session-store";
import type {
  SessionGoalCommandResult,
  SessionGoalSnapshot,
  SessionState,
} from "@/systems/session/types";
import { sessionKeys } from "../../lib/query-keys";

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

function renderHeader(
  setComposerText = vi.fn(),
  sessionState: SessionState = "active",
  readOnly = false
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  const header = (state: SessionState, stopping = false) => (
    <QueryClientProvider client={queryClient}>
      <SessionGoalHeaderContainer
        onPrefillComposer={setComposerText}
        sessionId={SESSION_ID}
        workspaceId={WORKSPACE_ID}
        sessionState={state}
        stopping={stopping}
        readOnly={readOnly}
      />
    </QueryClientProvider>
  );
  const result = render(header(sessionState));
  return {
    ...result,
    queryClient,
    setComposerText,
    setSessionLifecycle: (state: SessionState, stopping = false) =>
      result.rerender(header(state, stopping)),
  };
}

describe("SessionGoalHeader", () => {
  let visibleGoal: SessionGoalSnapshot | null;
  let goalReads: number;
  let commands: string[];
  let operations: string[];
  let controlResponse: (() => Promise<Response>) | null;
  let readFails: boolean;

  beforeEach(() => {
    visibleGoal = snapshot();
    goalReads = 0;
    commands = [];
    operations = [];
    controlResponse = null;
    readFails = false;
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
        if (
          pathname.endsWith(`/sessions/${SESSION_ID}/goal`) &&
          (init?.method ?? (input instanceof Request ? input.method : "GET")) === "POST"
        ) {
          const body = await requestBody(input, init);
          const operation = typeof body.operation === "string" ? body.operation : "";
          operations.push(operation);
          if (controlResponse) return controlResponse();
          if (operation === "pause") {
            visibleGoal = snapshot({ status: "paused", run_status: "paused" });
          }
          if (operation === "clear") {
            visibleGoal = null;
          }
          const outcome =
            operation === "pause" ? "paused" : operation === "clear" ? "cleared" : "status";
          return Response.json({
            outcome,
            reason_code: null,
            replaced_run_id: null,
            snapshot: visibleGoal,
          } satisfies SessionGoalCommandResult);
        }
        if (pathname.endsWith(`/sessions/${SESSION_ID}/goal`)) {
          goalReads += 1;
          if (readFails) return Response.json({ error: "Usage read failed" }, { status: 500 });
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

  it("Should merge live turns, invalidate only on status, and pause from the Goal surface", async () => {
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
    // The strip facts carry the merged turn count as a plain step fact.
    await waitFor(() =>
      expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent("Step 8 of 20")
    );
    expect(goalReads).toBe(1);

    act(() => {
      FakeEventSource.latest?.dispatch("goal_status_changed", { from: "active", to: "paused" }, 2);
    });
    await waitFor(() => expect(goalReads).toBe(2));

    // The action and its confirmed status stay in the same Goal surface.
    fireEvent.click(screen.getByTestId("goal-control-pause"));
    await waitFor(() => expect(operations).toContain("pause"));
    expect(commands).not.toContain("/goal pause");
    await waitFor(() =>
      expect(screen.getByRole("region", { name: "Goal status" })).toHaveAttribute(
        "data-goal-status",
        "paused"
      )
    );
    expect(screen.getByTestId("goal-control-resume")).toBeInTheDocument();
    expect(screen.queryByTestId("goal-control-pause")).not.toBeInTheDocument();
  });

  it("Should clear only a terminal snapshot from the Goal surface", async () => {
    visibleGoal = snapshot({ live: false, status: "complete", run_status: "done" });
    renderHeader();

    const clear = await screen.findByTestId("goal-control-clear");
    expect(screen.queryByTestId("goal-control-pause")).not.toBeInTheDocument();
    fireEvent.click(clear);

    await waitFor(() => expect(operations).toContain("clear"));
    expect(commands).not.toContain("/goal clear");
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

  // Invariant: accepted control stays pending until the authoritative snapshot says it is effective.
  // Owner: Goal mutation/read reconciliation; canonical suite: session-goal-header.
  it("Should distinguish a pending request, an accepted pause, and an effective pause", async () => {
    let release!: (response: Response) => void;
    controlResponse = () =>
      new Promise(resolve => {
        release = resolve;
      });
    renderHeader();
    fireEvent.click(await screen.findByRole("button", { name: "Pause goal" }));
    await waitFor(() => expect(operations).toEqual(["pause"]));
    expect(screen.getByTestId("goal-control-state")).toHaveTextContent("Pausing…");
    expect(screen.getByRole("button", { name: "Stop goal" })).toBeDisabled();
    visibleGoal = snapshot({ pause_requested: true });
    await act(async () =>
      release(
        Response.json({
          outcome: "paused",
          reason_code: null,
          replaced_run_id: null,
          snapshot: visibleGoal,
        })
      )
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "Stop goal" })).toBeEnabled());
    expect(screen.getByTestId("goal-control-state")).toHaveTextContent("Pausing…");
    expect(screen.queryByRole("button", { name: "Resume goal" })).not.toBeInTheDocument();
    visibleGoal = snapshot({ status: "paused", run_status: "paused" });
    act(() => FakeEventSource.latest?.dispatch("goal_status_changed", { to: "paused" }, 1));
    expect(await screen.findByRole("button", { name: "Resume goal" })).toBeEnabled();
    expect(screen.getByTestId("goal-control-state")).toHaveTextContent("Paused");
  });

  it("Should retain confirmed state and usable controls after a failed reread, then recover on retry", async () => {
    const { queryClient } = renderHeader();
    await screen.findByRole("button", { name: "Pause goal" });
    readFails = true;
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: sessionKeys.goal(WORKSPACE_ID, SESSION_ID) });
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't refresh the goal");
    expect(screen.getByRole("button", { name: "Pause goal" })).toBeEnabled();
    expect(screen.getByRole("button", { name: "Stop goal" })).toBeEnabled();
    readFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
  });

  it("Should surface control failures in place without announcing an effective pause", async () => {
    controlResponse = async () => Response.json({ error: "Control unavailable" }, { status: 503 });
    renderHeader();
    fireEvent.click(await screen.findByRole("button", { name: "Pause goal" }));
    expect(await screen.findByTestId("session-goal-control-error")).toBeVisible();
    expect(screen.getByTestId("goal-control-state")).toHaveTextContent("Active");
    expect(screen.getByRole("button", { name: "Pause goal" })).toBeEnabled();
  });

  it("Should stop through the typed Goal endpoint and keep the terminal Goal visible until Clear", async () => {
    controlResponse = async () => {
      visibleGoal = snapshot({ live: false, status: "paused", run_status: "canceled" });
      return Response.json({
        outcome: "stopped",
        reason_code: null,
        replaced_run_id: null,
        snapshot: visibleGoal,
      });
    };
    renderHeader();
    fireEvent.click(await screen.findByRole("button", { name: "Stop goal" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Stop goal" }));
    await waitFor(() => expect(operations).toEqual(["stop"]));
    expect(await screen.findByRole("button", { name: "Clear goal" })).toBeEnabled();
    expect(screen.getByTestId("goal-control-state")).toHaveTextContent("Stopped");
    expect(screen.queryByRole("button", { name: "Resume goal" })).not.toBeInTheDocument();
    expect(screen.getByTestId("session-goal-strip")).toBeVisible();
  });

  // Invariant: continuing a Goal requires an active session; session resume never resumes the Goal.
  // Owner: Goal header view-model; canonical suite: session-goal-header.
  it.each(["resume", "approve"] as const)(
    "Should keep %s unavailable until the session is active and require an explicit click",
    async action => {
      visibleGoal = snapshot({
        status: "paused",
        run_status: action === "approve" ? "needs-approval" : "paused",
      });
      const user = userEvent.setup();
      const { setSessionLifecycle } = renderHeader(vi.fn(), "stopped");
      const control = await screen.findByTestId(`goal-control-${action}`);
      expect(control).toHaveAttribute("aria-disabled", "true");
      expect(screen.getByRole("button", { name: "Stop goal" })).toBeEnabled();
      await user.tab();
      expect(control).toHaveFocus();
      expect(await screen.findByRole("tooltip")).toHaveTextContent(
        "Send a message to reopen this session, then resume the goal."
      );
      expect(control).toHaveAttribute("aria-describedby", screen.getByRole("tooltip").id);
      await user.keyboard("{Enter}");
      fireEvent.click(control);
      expect(operations).toEqual([]);

      setSessionLifecycle("stopping");
      expect(control).toHaveAttribute("aria-disabled", "true");
      fireEvent.click(control);
      expect(operations).toEqual([]);

      setSessionLifecycle("active", true);
      expect(control).toHaveAttribute("aria-disabled", "true");
      fireEvent.click(control);
      expect(operations).toEqual([]);

      setSessionLifecycle("active");
      expect(control).not.toHaveAttribute("aria-disabled");
      expect(operations).toEqual([]);
      fireEvent.click(control);
      await waitFor(() => expect(operations).toEqual(["resume"]));
    }
  );

  // Invariant: read-only transcripts expose Goal state and rereads, never Goal mutations.
  // Owner: Goal header view-model; canonical suite: session-goal-header.
  it("Should omit every Goal mutation in read-only mode while retaining state and Retry", async () => {
    const { queryClient } = renderHeader(vi.fn(), "active", true);
    await screen.findByRole("region", { name: "Goal status" });
    const states: Partial<SessionGoalSnapshot>[] = [
      {},
      { status: "paused", run_status: "paused" },
      { run_status: "needs-approval" },
      { live: false, status: "complete", run_status: "done" },
    ];
    for (const state of states) {
      visibleGoal = snapshot(state);
      await act(async () => {
        await queryClient.invalidateQueries({
          queryKey: sessionKeys.goal(WORKSPACE_ID, SESSION_ID),
        });
      });
      expect(screen.getByRole("region", { name: "Goal status" })).toBeVisible();
      expect(screen.queryByRole("group", { name: "Goal controls" })).not.toBeInTheDocument();
    }
    readFails = true;
    await act(async () => {
      await queryClient.invalidateQueries({ queryKey: sessionKeys.goal(WORKSPACE_ID, SESSION_ID) });
    });
    expect(await screen.findByRole("alert")).toHaveTextContent("Couldn't refresh the goal");
    readFails = false;
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.queryByRole("alert")).not.toBeInTheDocument());
    expect(operations).toEqual([]);
    expect(commands).toEqual([]);
  });
});
