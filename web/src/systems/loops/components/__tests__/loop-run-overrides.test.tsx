import { useState } from "react";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { LoopRunOverrides } from "../run-form/loop-run-overrides";
import { initialOverrideDraft, type LoopOverrideDraft } from "../../lib/loop-overrides";
import { loopEffectiveConfigFixture } from "../../mocks/fixtures";

function Harness() {
  const [draft, setDraft] = useState<LoopOverrideDraft>(() =>
    initialOverrideDraft(loopEffectiveConfigFixture)
  );
  return (
    <LoopRunOverrides
      effectiveConfig={loopEffectiveConfigFixture}
      draft={draft}
      onChange={setDraft}
    />
  );
}

/** Expand the Advanced collapsible so its clamped grid mounts. */
function openAdvanced() {
  fireEvent.click(screen.getByRole("button", { name: /Advanced/ }));
}

describe("LoopRunOverrides", () => {
  it("Should expose the 6 clamped fields and no cost-cap field", () => {
    render(<Harness />);
    openAdvanced();
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("using loop defaults");
    expect(screen.getByTestId("loop-run-override-input-iteration_cap")).toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-override-cost")).not.toBeInTheDocument();
    expect(screen.queryByTestId("loop-run-override-budget_usd")).not.toBeInTheDocument();
    expect(screen.getByTestId("loop-run-overrides")).not.toHaveTextContent(/cost cap/i);
  });

  it("Should clamp an over-ceiling override and flip the overrides-set badge", () => {
    render(<Harness />);
    openAdvanced();
    const input = screen.getByTestId("loop-run-override-input-iteration_cap");
    fireEvent.change(input, { target: { value: "150" } });
    expect(input).toHaveValue(100);
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("overrides set");
  });

  it("Should return to 'using loop defaults' when a field is cleared", () => {
    render(<Harness />);
    openAdvanced();
    const input = screen.getByTestId("loop-run-override-input-gate_max_revisions");
    fireEvent.change(input, { target: { value: "9" } });
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("overrides set");
    fireEvent.change(input, { target: { value: "" } });
    expect(screen.getByTestId("loop-run-overrides-badge")).toHaveTextContent("using loop defaults");
  });
});
