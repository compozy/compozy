import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { SessionGoalSnapshot } from "@/systems/session/types";

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

const fetchSessions = vi.hoisted(() => vi.fn());
vi.mock("../../adapters/session-api", async importOriginal => ({
  ...(await importOriginal<typeof import("../../adapters/session-api")>()),
  fetchSessions,
}));

const { SessionGoalStrip } = await import("../goal/session-goal-strip");

function snapshot(overrides: Partial<SessionGoalSnapshot> = {}): SessionGoalSnapshot {
  return {
    bound_session_id: "session_1",
    cause: null,
    context: {
      nudge_ratio: 0.7,
      ratio: 0.41,
      reported_at: "2026-07-10T12:00:00Z",
      size: 200_000,
      state: "known",
      used: 84_120,
    },
    contract_summary: "All 16 transcript components on session.css tokens.",
    last_verdict: null,
    live: true,
    node_id: "goal",
    objective: "Migrate the session transcript",
    origin_session_id: "session_1",
    run_id: "run_1",
    run_status: "running",
    status: "active",
    turn_limit: 20,
    turns_used: 3,
    ...overrides,
  };
}

describe("SessionGoalStrip", () => {
  // Invariant: the audit list holds its place with placeholder lines while it loads, then hands
  // them to the real links. Owner: the goal strip's expanded body, canonical suite below.
  it("Should hold the audit list's place with placeholder lines until it arrives", async () => {
    let release: (page: unknown) => void = () => undefined;
    fetchSessions.mockReturnValue(
      new Promise(resolve => {
        release = resolve;
      })
    );
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <SessionGoalStrip snapshot={snapshot()} />
      </QueryClientProvider>
    );
    fireEvent.click(screen.getByTestId("goal-strip-line"));
    fireEvent.click(screen.getByRole("button", { name: "Verification sessions" }));

    const loading = await screen.findByRole("status");
    expect(loading).toHaveTextContent("Loading verification sessions");
    expect(loading.querySelectorAll('[data-slot="skeleton-text"]').length).toBeGreaterThan(0);

    release({
      sessions: [{ id: "sess-judge", name: "Verification judge", state: "stopped" }],
      page: { total: 1, limit: 50, has_more: false },
    });
    expect(await screen.findByRole("link", { name: "Verification judge · stopped" })).toBeVisible();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    client.clear();
  });

  it("Should load the bound session's explicit system audit and open a judge transcript", async () => {
    fetchSessions.mockResolvedValue({
      sessions: [{ id: "sess-judge", name: "Verification judge", state: "stopped" }],
      page: { total: 1, limit: 50, has_more: false },
    });
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <SessionGoalStrip snapshot={snapshot()} />
      </QueryClientProvider>
    );
    fireEvent.click(screen.getByTestId("goal-strip-line"));
    expect(fetchSessions).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Verification sessions" }));
    const link = await screen.findByRole("link", { name: "Verification judge · stopped" });
    expect(link).toHaveAttribute("data-params", JSON.stringify({ id: "sess-judge" }));
    expect(fetchSessions).toHaveBeenCalledWith(
      expect.objectContaining({ parent: "session_1", type: "system", all_profiles: true }),
      expect.any(AbortSignal)
    );
    client.clear();
  });

  it.each([
    ["active", "running", "active"],
    ["paused", "paused", "paused"],
    ["blocked", "blocked", "blocked"],
    ["usage-limited", "stalled", "blocked"],
    ["complete", "done", "done"],
  ] as const)("Should map status %s to strip state %s", (status, runStatus, expected) => {
    render(<SessionGoalStrip snapshot={snapshot({ status, run_status: runStatus })} />);
    expect(screen.getByTestId("session-goal-strip")).toHaveAttribute("data-state", expected);
  });

  it("Should render the plain step fact, leaving context usage to the composer ring", () => {
    render(<SessionGoalStrip snapshot={snapshot()} />);
    expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent("Step 3 of 20");
    expect(screen.getByTestId("goal-strip-facts")).not.toHaveTextContent("ctx");
  });

  it.each(["pending", "unknown"] as const)(
    "Should explain %s context in the body without inventing a percentage",
    state => {
      render(
        <SessionGoalStrip
          snapshot={snapshot({
            context: {
              nudge_ratio: 0.7,
              ratio: null,
              reported_at: null,
              size: null,
              state,
              used: null,
            },
          })}
        />
      );
      expect(screen.getByTestId("goal-strip-facts")).not.toHaveTextContent("%");

      fireEvent.click(screen.getByTestId("goal-strip-line"));
      expect(screen.getByTestId("goal-strip-body")).toHaveTextContent(
        state === "pending"
          ? "Waiting for a newer usage report."
          : "This agent has not reported context usage."
      );
    }
  );

  it("Should expand to the quiet body rows: contract, run link, context tokens; node facts stay off screen", () => {
    render(<SessionGoalStrip snapshot={snapshot({ cause: "goal_agent_refused" })} />);
    const line = screen.getByTestId("goal-strip-line");
    expect(line).toHaveAttribute("aria-expanded", "false");

    fireEvent.click(line);

    const body = screen.getByTestId("goal-strip-body");
    expect(body).toHaveTextContent("All 16 transcript components on session.css tokens.");
    expect(body).toHaveTextContent("84,120 / 200,000 tokens · nudge at 70%");
    // Node and cause are diagnostics: attributes, never body text.
    expect(body).not.toHaveTextContent("goal_agent_refused");
    const strip = screen.getByTestId("session-goal-strip");
    expect(strip).toHaveAttribute("data-goal-node", "goal");
    expect(strip).toHaveAttribute("data-goal-cause", "goal_agent_refused");
    const runLink = screen.getByRole("link", { name: "Open run" });
    expect(runLink).toHaveAttribute("data-params", JSON.stringify({ runId: "run_1" }));
    expect(runLink).toHaveAttribute("data-hash", "node-goal");
  });

  it("Should render the moved variant with the faint dot and active-session link", () => {
    render(
      <SessionGoalStrip
        snapshot={snapshot({ bound_session_id: "session_2", origin_session_id: "session_1" })}
      />
    );
    const strip = screen.getByTestId("session-goal-strip");
    // Moving sessions must not hide the Goal's actual lifecycle state.
    expect(strip).toHaveAttribute("data-state", "active");
    expect(screen.getByTestId("goal-control-state")).toHaveTextContent("Active");
    expect(screen.getByTestId("goal-strip-facts")).toHaveTextContent(/^Moved · /);

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    const sessionLink = screen.getByTestId("goal-strip-active-session");
    expect(sessionLink).toHaveAttribute("data-params", JSON.stringify({ id: "session_2" }));
  });

  it("Should stage the draft goal command into the composer without sending", () => {
    const onPrefillComposer = vi.fn();
    render(
      <SessionGoalStrip
        snapshot={snapshot()}
        composerAffordance={{ kind: "draft", expandedObjective: "Ship the calm transcript" }}
        onPrefillComposer={onPrefillComposer}
      />
    );

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    fireEvent.click(screen.getByRole("button", { name: "Draft goal command" }));
    expect(onPrefillComposer).toHaveBeenCalledWith("/goal Ship the calm transcript");
  });

  it("Should stage an exact run-bound replacement command into the composer", () => {
    const onPrefillComposer = vi.fn();
    render(
      <SessionGoalStrip
        snapshot={snapshot()}
        composerAffordance={{
          kind: "replace",
          expectedRunId: "run_expected",
          objective: "Ship the calm transcript",
        }}
        onPrefillComposer={onPrefillComposer}
      />
    );

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    fireEvent.click(screen.getByRole("button", { name: "Draft replacement" }));
    expect(onPrefillComposer).toHaveBeenCalledWith(
      "/goal replace run_expected Ship the calm transcript"
    );
  });

  it("Should render the last verdict as quiet body text with mono evidence", () => {
    render(
      <SessionGoalStrip
        snapshot={snapshot({
          last_verdict: {
            blocking_issues: [
              { id: "issue_1", note: "a" },
              { id: "issue_2", note: "b" },
            ],
            evaluated_at: "2026-07-10T12:00:00Z",
            evidence_ref: "verdict_007",
            outcome: "approved",
          },
        })}
      />
    );

    fireEvent.click(screen.getByTestId("goal-strip-line"));
    const body = screen.getByTestId("goal-strip-body");
    expect(body).toHaveTextContent("Approved · 2 blocking issues");
    expect(body).toHaveTextContent("verdict_007");
  });

  // Invariant: the full objective remains in the keyboard-accessible disclosure, independent of the contract summary.
  // Owner: session Goal disclosure; canonical suite: session-goal-strip.
  it("Should disclose the exact multiline Goal objective", () => {
    const objective = "Inspect summary layout\n" + "unbroken_path_".repeat(80) + "objective-tail";
    render(<SessionGoalStrip snapshot={snapshot({ objective })} />);
    fireEvent.click(screen.getByTestId("goal-strip-line"));
    const body = screen.getByTestId("goal-strip-body");
    expect(body.textContent).toContain(objective);
  });
});
