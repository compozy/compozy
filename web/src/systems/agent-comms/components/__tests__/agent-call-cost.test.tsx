import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AgentCallCost } from "../agent-call-cost";

/**
 * The invariant: call cost is whatever `describeCost()` says, character for
 * character. This surface owns no cost wording of its own, so the failure this
 * suite guards against is a re-derivation creeping in — a local currency format,
 * a friendlier word for "Unavailable", or the classic one, rendering absent
 * provider data as `0`.
 */
describe("AgentCallCost", () => {
  it("Should mark an estimate with the approximate glyph and name it as estimated", () => {
    render(
      <AgentCallCost
        data-testid="cost"
        usage={{
          status: "estimated",
          source: "models_dev",
          amount: 0.038,
          currency: "USD",
        }}
      />
    );

    expect(screen.getByText("≈ $0.038")).toBeInTheDocument();
    expect(screen.getByText("Estimated · models.dev rate")).toBeInTheDocument();
  });

  it("Should render an actual amount without the approximate glyph", () => {
    render(
      <AgentCallCost
        usage={{ status: "actual", source: "agent_reported", amount: 0.18, currency: "USD" }}
      />
    );

    expect(screen.getByText("$0.180")).toBeInTheDocument();
    expect(screen.queryByText(/≈/)).not.toBeInTheDocument();
  });

  it("Should say Unavailable when the provider reported nothing", () => {
    render(<AgentCallCost data-testid="cost" usage={{ status: "unknown" }} />);

    expect(screen.getByText("Unavailable")).toBeInTheDocument();
    expect(screen.getByText("Cost unavailable")).toBeInTheDocument();
    // The failure this exists to prevent.
    expect(screen.queryByText("0")).not.toBeInTheDocument();
    expect(screen.queryByText("$0.00")).not.toBeInTheDocument();
  });

  it("Should render an em dash when the daemon declared no cost status at all", () => {
    render(<AgentCallCost data-testid="cost" usage={{}} />);

    expect(screen.getByText("—")).toBeInTheDocument();
  });

  it("Should say Included rather than a zero amount under a subscription", () => {
    render(<AgentCallCost usage={{ status: "included" }} />);

    expect(screen.getByText("Included")).toBeInTheDocument();
    expect(screen.getByText("Subscription")).toBeInTheDocument();
  });
});
