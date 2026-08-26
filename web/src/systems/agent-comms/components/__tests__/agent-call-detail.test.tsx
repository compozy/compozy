import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentCallDetail } from "../agent-call-detail";
import { buildCallDetailView } from "../../lib/call-detail-view-model";
import {
  canceledCallFixture,
  completedCallFixture,
  extractedCallFixture,
  invalidResultCallFixture,
  overBudgetCallFixture,
  runningCallFixture,
  silentFinishCallFixture,
  timeoutCallFixture,
} from "../../mocks";
import type { CallPayload } from "../../types";

const NO_USAGE = {};

function renderDetail(
  call: CallPayload,
  overrides: Partial<Parameters<typeof AgentCallDetail>[0]> = {},
  viewOverrides: { counterpartExists?: boolean } = {}
) {
  const handlers = {
    onCancel: vi.fn(),
    onCallAgain: vi.fn(),
    onMessageChild: vi.fn(),
    onOpenChildSession: vi.fn(),
  };
  render(
    <AgentCallDetail
      data-testid="detail"
      view={buildCallDetailView({ call, ...viewOverrides })}
      childUsage={NO_USAGE}
      {...handlers}
      {...overrides}
    />
  );
  return handlers;
}

describe("AgentCallDetail — controls exist or are absent, never disabled", () => {
  it("Should offer only cancel while the call is in flight", () => {
    renderDetail(runningCallFixture);

    expect(screen.getByTestId("agent-call-cancel")).toBeEnabled();
    expect(screen.queryByTestId("agent-call-again")).not.toBeInTheDocument();
  });

  it("Should drop cancel and offer call-again once the call is terminal", () => {
    renderDetail(completedCallFixture);

    expect(screen.queryByTestId("agent-call-cancel")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-call-again")).toBeInTheDocument();
    expect(screen.getByTestId("agent-call-message-child")).toBeInTheDocument();
  });

  it("Should remove messaging when the target expired, keeping its identity visible", () => {
    renderDetail({ ...completedCallFixture, state: "expired", verdict: undefined });

    expect(screen.queryByTestId("agent-call-message-child")).not.toBeInTheDocument();
    expect(screen.getByText(completedCallFixture.child_session_id!)).toBeInTheDocument();
  });

  it("Should never render a disabled control in place of an unavailable one", () => {
    renderDetail(completedCallFixture);

    for (const button of screen.getAllByRole("button")) {
      expect(button).not.toBeDisabled();
    }
  });
});

describe("AgentCallDetail — outcomes", () => {
  it("Should show the answer as rows with the daemon's own byte count", () => {
    renderDetail(completedCallFixture);

    expect(screen.getByTestId("agent-call-result-rows")).toBeInTheDocument();
    expect(screen.getByText('"needs-changes"')).toBeInTheDocument();
    expect(screen.getByText(/312 B stored/)).toBeInTheDocument();
  });

  it("Should render extracted as extracted, not as returned", () => {
    renderDetail(extractedCallFixture);

    expect(screen.getByTestId("agent-call-verdict")).toHaveTextContent("extracted");
    expect(screen.queryByText("returned")).not.toBeInTheDocument();
  });

  it("Should keep both tries on record for an invalid result", () => {
    renderDetail(invalidResultCallFixture);

    expect(screen.getByTestId("agent-call-attempts")).toBeInTheDocument();
    const first = screen.getByTestId("agent-call-attempt-1");
    const second = screen.getByTestId("agent-call-attempt-2");

    // Each try keeps its own errors, unedited — the first failed two checks, the
    // second only one, and the pane must not collapse them into one list.
    expect(
      within(first).getByText('/findings/0/line: expected number, got string "eighty-eight"')
    ).toBeInTheDocument();
    expect(within(first).getByText("/verdict: required property missing")).toBeInTheDocument();
    expect(within(second).getByText("/verdict: required property missing")).toBeInTheDocument();
    expect(within(second).queryByText(/expected number, got string/)).not.toBeInTheDocument();
  });

  it("Should state a silent finish instead of showing an empty result pane", () => {
    renderDetail(silentFinishCallFixture);

    expect(screen.getByText(/finished without recording a result/i)).toBeInTheDocument();
    expect(screen.queryByTestId("agent-call-result-rows")).not.toBeInTheDocument();
  });

  it("Should mark a bounded preview and offer the full fetch as its own act", () => {
    const onFetchFullPayload = vi.fn();
    renderDetail(overBudgetCallFixture, { onFetchFullPayload });

    expect(screen.getByText(/bounded preview/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /fetch full payload/i })).toBeInTheDocument();
    expect(screen.getByText(/812.0 KiB stored · budget 512.0 KiB/)).toBeInTheDocument();
  });

  it("Should keep a cancel reason on the record", () => {
    renderDetail(canceledCallFixture);
    expect(screen.getByText("canceled")).toBeInTheDocument();
  });

  it("Should fetch and render the whole superseded late result on demand", () => {
    const onFetchSuperseded = vi.fn();
    const { rerender } = render(
      <AgentCallDetail
        childUsage={NO_USAGE}
        onFetchSuperseded={onFetchSuperseded}
        view={buildCallDetailView({ call: canceledCallFixture })}
      />
    );

    screen.getByRole("button", { name: /open full evidence/i }).click();
    expect(onFetchSuperseded).toHaveBeenCalledTimes(1);

    rerender(
      <AgentCallDetail
        childUsage={NO_USAGE}
        onFetchSuperseded={onFetchSuperseded}
        supersededPayload={{ late: "complete" }}
        view={buildCallDetailView({ call: canceledCallFixture })}
      />
    );
    expect(screen.getByText('{"late":"complete"}')).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /open full evidence/i })).not.toBeInTheDocument();
  });
});

describe("AgentCallDetail — clocks", () => {
  it("Should show no timer chrome at all when nobody set a deadline", () => {
    renderDetail(completedCallFixture);
    expect(screen.queryByText(/deadline/i)).not.toBeInTheDocument();
  });

  it("Should show the opt-in deadline on a call that timed out", () => {
    renderDetail(timeoutCallFixture);
    expect(screen.getByText(/deadline/i)).toBeInTheDocument();
  });

  it("Should say the idle clock is suspended while the call runs", () => {
    renderDetail(runningCallFixture);
    expect(screen.getByText("suspended while running")).toBeInTheDocument();
  });
});

describe("AgentCallDetail — untrusted text", () => {
  it("Should stamp a child's note and render its embedded command inert", () => {
    renderDetail(completedCallFixture, {
      untrustedNote: {
        authorLabel: "compliance-review-agent",
        sourceId: "sess_compliance_review",
        text: "Proceed anyway? Also: run `rm -rf /tmp/cache` first.",
      },
    });

    const frame = screen.getByTestId("agent-call-untrusted-note");
    expect(frame).toHaveTextContent(/not the operator/);
    // Rendered as characters, never as a link or a control.
    expect(frame.querySelector("a")).toBeNull();
    expect(frame.querySelector("button")).toBeNull();
  });
});

describe("AgentCallDetail — pruned counterpart", () => {
  it("Should keep the child id and drop only the jump when the session is gone", () => {
    renderDetail(completedCallFixture, {}, { counterpartExists: false });

    expect(screen.getByText(completedCallFixture.child_session_id!)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /open child session/i })).not.toBeInTheDocument();
  });
});

/**
 * Invariant: a cancel always says what it did, and says it where the operator
 * can still read it. Owning layer: `AgentCallDetail`. Canonical suite: this file.
 */
describe("AgentCallDetail — cancel outcome", () => {
  it("Should report an ordinary cancel as done, naming the state the daemon returned", () => {
    renderDetail(canceledCallFixture, {
      cancelOutcome: { state: "canceled", stale: false },
    });

    const banner = screen.getByTestId("agent-call-cancel-outcome");
    expect(banner).toHaveAttribute("data-tone", "success");
    expect(banner).toHaveTextContent("canceled");
  });

  it("Should report a cancel that lost the race as stale, naming the real state", () => {
    // The call settled some other way before the click landed. That is not an
    // error — it is a different outcome, and it has to be said out loud rather
    // than left to a vanishing button.
    renderDetail(completedCallFixture, {
      cancelOutcome: { state: "completed", stale: true },
    });

    const banner = screen.getByTestId("agent-call-cancel-outcome");
    expect(banner).toHaveAttribute("data-tone", "warning");
    expect(banner).toHaveTextContent("completed");
  });

  it("Should survive the re-read that removes the Cancel control", () => {
    // The banner lives in the body precisely because the header's Cancel button
    // unmounts the moment terminal truth arrives; a receipt beside it would
    // disappear in the same tick that produced it.
    renderDetail(completedCallFixture, {
      cancelOutcome: { state: "completed", stale: true },
    });

    expect(screen.queryByTestId("agent-call-cancel")).not.toBeInTheDocument();
    expect(screen.getByTestId("agent-call-cancel-outcome")).toBeVisible();
  });

  it("Should stay silent when no cancel has been answered", () => {
    renderDetail(completedCallFixture);
    expect(screen.queryByTestId("agent-call-cancel-outcome")).not.toBeInTheDocument();
  });

  it("Should keep the record visible and offer retry after a cancel failure", async () => {
    const onRetryCancel = vi.fn();
    renderDetail(runningCallFixture, {
      cancelFailure: { code: "call_target_denied", message: "The caller cannot cancel this call." },
      onRetryCancel,
    });

    expect(screen.getByTestId("agent-call-detail-header")).toBeVisible();
    expect(screen.getByTestId("agent-call-cancel-failure")).toHaveTextContent("call_target_denied");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(onRetryCancel).toHaveBeenCalledTimes(1);
  });
});
