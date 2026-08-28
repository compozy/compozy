import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

  it("Should open a selected row and load the next page", async () => {
    const user = userEvent.setup();
    const onOpenCall = vi.fn();
    const made = section([completedCallFixture], 2, true);
    render(
      <AgentCallsInspectorPanel made={made} received={section([], 0)} onOpenCall={onOpenCall} />
    );

    await user.click(screen.getByRole("button", { name: completedCallFixture.agent ?? "" }));
    await user.click(screen.getByTestId("agent-calls-panel-made-more"));

    expect(onOpenCall).toHaveBeenCalledWith(completedCallFixture.call_id);
    expect(made.onLoadMore).toHaveBeenCalledOnce();
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
    expect(screen.queryByText(/When this session delegates work/)).not.toBeInTheDocument();
  });

  it("Should hide an empty direction when the other side has calls", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], 1)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByTestId("agent-calls-panel-made")).toBeInTheDocument();
    expect(screen.queryByTestId("agent-calls-panel-received")).not.toBeInTheDocument();
  });

  it("Should name a daemon-shaped human Received row as operator", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([], 0)}
        received={section(
          [
            {
              ...completedCallFixture,
              actor: { id: "operator:http", kind: "human" },
              caller: { id: "ses_operator_hidden", kind: "session" },
            },
          ],
          1
        )}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "operator" })).toBeInTheDocument();
    expect(screen.queryByText("ses_operator_hidden")).not.toBeInTheDocument();
  });

  it("Should name an agent Received row from the session catalog", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([], 0)}
        received={section(
          [
            {
              ...completedCallFixture,
              actor: { id: "ses_reviewer", kind: "agent_session" },
              caller: { id: "ses_reviewer", kind: "session" },
            },
          ],
          1
        )}
        callerNames={new Map([["ses_reviewer", "Review lead"]])}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "Review lead" })).toBeInTheDocument();
    expect(screen.queryByText("ses_reviewer")).not.toBeInTheDocument();
  });

  it("Should name a Made row without an agent instead of showing a session id", () => {
    render(
      <AgentCallsInspectorPanel
        made={section([{ ...completedCallFixture, agent: undefined }], 1)}
        received={section([], 0)}
        onOpenCall={vi.fn()}
      />
    );

    expect(screen.getByRole("button", { name: "unknown agent" })).toBeInTheDocument();
    expect(screen.queryByText(completedCallFixture.child_session_id ?? "")).not.toBeInTheDocument();
  });
});

describe("AgentCallsInspectorPanel — pruned counterpart", () => {
  it("Should keep the record openable and drop only the session hop when the counterpart is gone", async () => {
    const user = userEvent.setup();
    const onOpenCall = vi.fn();
    render(
      <AgentCallsInspectorPanel
        made={section([completedCallFixture], 1)}
        received={section([], 0)}
        onOpenCall={onOpenCall}
        prunedSessionIds={new Set([completedCallFixture.child_session_id!])}
      />
    );

    const row = screen.getByTestId("agent-calls-panel-row");
    expect(row).toHaveAttribute("data-pruned", "true");
    expect(row.tagName).toBe("BUTTON");
    expect(within(row).getByText("session pruned — record retained")).toBeInTheDocument();
    expect(within(row).getByText("completed")).toBeInTheDocument();

    await user.click(row);
    expect(onOpenCall).toHaveBeenCalledWith(completedCallFixture.call_id);
  });
});
