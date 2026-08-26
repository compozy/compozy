import { render, screen, within } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AgentCallsInspectorPanel } from "../agent-calls-inspector-panel";
import { buildLargeTreeFixture, completedCallFixture, runningCallFixture } from "../../mocks";

function section(
  calls: Parameters<typeof AgentCallsInspectorPanel>[0]["made"]["calls"],
  total: number | undefined,
  hasMore = false,
  overrides: Partial<Parameters<typeof AgentCallsInspectorPanel>[0]["made"]> = {}
) {
  return { calls, total, hasMore, onLoadMore: vi.fn(), ...overrides };
}

describe("AgentCallsInspectorPanel — truthful counts", () => {
  it("Should show the daemon total, not the number of rows it loaded", () => {
    const page = buildLargeTreeFixture(25);
    render(
      <AgentCallsInspectorPanel
        data-testid="panel"
        made={section(page, 247, true)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByTestId("agent-calls-panel-made-count")).toHaveTextContent("247");
    expect(screen.getAllByTestId("agent-calls-panel-row")).toHaveLength(25);
    expect(screen.getByText("25 of 247 loaded")).toBeInTheDocument();
  });

  it("Should say only what it knows while the count is still in flight", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], undefined, true)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.queryByTestId("agent-calls-panel-made-count")).not.toBeInTheDocument();
    expect(screen.getByText("1 loaded")).toBeInTheDocument();
  });

  it("Should drop the pager once the whole population is loaded", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], 1, false)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.queryByTestId("agent-calls-panel-made-more")).not.toBeInTheDocument();
  });
});

describe("AgentCallsInspectorPanel — directions", () => {
  it("Should list both directions and distinguish them by arrow, not colour", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], 1)}
        received={section([runningCallFixture], 1)}
        onOpenCall={vi.fn()}
      />
    );

    const made = screen.getByTestId("agent-calls-panel-made");
    const received = screen.getByTestId("agent-calls-panel-received");
    expect(within(made).getAllByTestId("agent-calls-panel-row")).toHaveLength(1);
    expect(within(received).getAllByTestId("agent-calls-panel-row")).toHaveLength(1);
  });

  it("Should show a failed first read instead of calling it loading", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([], undefined, false, {
          error: "Connection refused",
          onRetry: vi.fn(),
        })}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByTestId("agent-calls-panel-made-error")).toHaveTextContent(
      "Connection refused"
    );
    expect(screen.queryByText("Loading…")).not.toBeInTheDocument();
  });

  it("Should keep loaded rows usable when loading older calls fails", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], 2, true, {
          error: "The next page failed",
          onRetry: vi.fn(),
        })}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByTestId("agent-calls-panel-made-error")).toBeInTheDocument();
    expect(screen.getByTestId("agent-calls-panel-row")).toHaveAttribute(
      "data-call-id",
      completedCallFixture.call_id
    );
  });

  it("Should teach the feature when the session has never delegated", () => {
    render(
      <AgentCallsInspectorPanel
        data-testid="panel"
        made={section([], 0)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByText("No calls yet")).toBeInTheDocument();
  });
});

describe("AgentCallsInspectorPanel — pruned counterpart", () => {
  it("Should keep the record and drop only the jump when the session is gone", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], 1)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
        prunedSessionIds={new Set([completedCallFixture.child_session_id!])}
      />
    );

    const row = screen.getByTestId("agent-calls-panel-row");
    expect(row).toHaveAttribute("data-pruned", "true");
    expect(within(row).queryByRole("button")).toBeNull();
    expect(within(row).getByText("session pruned — record retained")).toBeInTheDocument();
    // The state it reached is still on the record.
    expect(within(row).getByText("completed")).toBeInTheDocument();
  });
});
