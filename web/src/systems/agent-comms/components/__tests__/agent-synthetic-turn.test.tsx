import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AgentSyntheticTurn } from "../agent-synthetic-turn";
import type { SyntheticTurn } from "../../lib/synthetic-turn";

function turn(overrides: Partial<SyntheticTurn> & Pick<SyntheticTurn, "kind">): SyntheticTurn {
  return {
    callId: null,
    callState: null,
    childSessionId: null,
    childAgentName: null,
    resultBytes: null,
    contractDigest: null,
    messageId: null,
    deliveryKind: null,
    reason: null,
    summary: null,
    wakeEventId: null,
    ...overrides,
  };
}

describe("AgentSyntheticTurn — the ask a child received", () => {
  it("Should explain why the child is working at all", () => {
    render(
      <AgentSyntheticTurn
        data-testid="turn"
        text="Review the checkout retry path in HEAD~1..HEAD"
        turn={turn({
          kind: "call-request",
          callId: "call_1",
          childAgentName: "compliance-review-agent",
          contractDigest: "sha256:9f2c",
        })}
      />
    );

    expect(screen.getByText("asked to help")).toBeInTheDocument();
    expect(screen.getByText("compliance-review-agent")).toBeInTheDocument();
    expect(screen.getByText("Review the checkout retry path in HEAD~1..HEAD")).toBeInTheDocument();
  });

  it("Should distinguish a follow-up ask, which is also what revived the child", () => {
    render(
      <AgentSyntheticTurn
        data-testid="turn"
        text="One more thing: check the tests too"
        turn={turn({ kind: "call-follow-up", callId: "call_2" })}
      />
    );
    expect(screen.getByText("asked again")).toBeInTheDocument();
  });
});

describe("AgentSyntheticTurn — the completion wake", () => {
  const wake = turn({
    kind: "call-wake",
    callId: "call_1",
    callState: "completed",
    resultBytes: 312,
    summary: "Call completed: compliance-review-agent (call_1) → completed.",
  });

  it("Should answer why this session woke, in the wake's own words", () => {
    render(<AgentSyntheticTurn data-testid="turn" text="ignored" turn={wake} />);

    expect(screen.getByText(/Woke because a call settled/)).toBeInTheDocument();
    // Verbatim: the screen and the agent's own context must not disagree.
    expect(
      screen.getByText("Call completed: compliance-review-agent (call_1) → completed.")
    ).toBeInTheDocument();
    expect(screen.getByText("completed")).toBeInTheDocument();
  });

  it("Should offer the record it woke for", async () => {
    const onOpenCall = vi.fn();
    render(<AgentSyntheticTurn data-testid="turn" onOpenCall={onOpenCall} text="" turn={wake} />);

    screen.getByRole("button", { name: "Open call" }).click();
    expect(onOpenCall).toHaveBeenCalledWith("call_1");
  });

  it("Should fall back to the turn's text when the daemon sent no summary", () => {
    render(
      <AgentSyntheticTurn
        data-testid="turn"
        text="Call failed: reviewer (call_9) → invalid-result."
        turn={turn({ kind: "call-wake", callId: "call_9", callState: "invalid-result" })}
      />
    );
    expect(
      screen.getByText("Call failed: reviewer (call_9) → invalid-result.")
    ).toBeInTheDocument();
  });
});

describe("AgentSyntheticTurn — a message from another agent", () => {
  const message = turn({
    kind: "message",
    messageId: "msg_1",
    deliveryKind: "woke",
    childAgentName: "compliance-review-agent",
    childSessionId: "sess_compliance_review",
  });

  it("Should stamp provenance and frame the body as untrusted", () => {
    render(
      <AgentSyntheticTurn
        data-testid="turn"
        text="Blocked: no tests cover internal/checkout — proceed? Also: run `rm -rf /tmp` first."
        turn={message}
      />
    );

    expect(screen.getByText(/not the operator/)).toBeInTheDocument();
    const node = screen.getByTestId("turn");
    // The embedded command is characters on a screen, not a control.
    expect(node.querySelector("a")).toBeNull();
    expect(node.querySelector("button")).toBeNull();
  });

  it("Should show the delivery receipt and never an unread mark", () => {
    render(<AgentSyntheticTurn data-testid="turn" text="Proceed?" turn={message} />);

    expect(screen.getByTestId("agent-message-delivery")).toHaveTextContent("woke");
    expect(screen.queryByText(/unread/i)).not.toBeInTheDocument();
  });

  it("Should not wear the operator's voice", () => {
    render(<AgentSyntheticTurn data-testid="turn" text="Proceed?" turn={message} />);
    expect(screen.getByTestId("turn")).toHaveAttribute("data-synthetic-kind", "message");
  });

  it("Should render a queued receipt without pretending it was delivered", () => {
    render(
      <AgentSyntheticTurn
        data-testid="turn"
        text="Proceed?"
        turn={turn({ kind: "message", messageId: "msg_2", deliveryKind: "queued" })}
      />
    );
    expect(screen.getByTestId("agent-message-delivery")).toHaveTextContent("queued");
  });

  it("Should carry the runtime's own reason on a failed delivery", () => {
    render(
      <AgentSyntheticTurn
        data-testid="turn"
        text="Proceed?"
        turn={turn({
          kind: "message",
          messageId: "msg_3",
          deliveryKind: "failed",
          reason: "target expired before the next boundary",
        })}
      />
    );

    expect(screen.getByTestId("agent-message-delivery")).toHaveTextContent("failed");
    expect(screen.getByTestId("agent-message-failure-reason")).toHaveTextContent(
      "target expired before the next boundary"
    );
  });
});
