import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Stepper, StepperItem, StepperTrigger } from "../stepper";

function StepperFixture({ includeMiddle = true }: { includeMiddle?: boolean }) {
  return (
    <Stepper defaultValue={1}>
      <StepperItem step={1}>
        <StepperTrigger>First</StepperTrigger>
      </StepperItem>
      {includeMiddle ? (
        <StepperItem step={2}>
          <StepperTrigger>Second</StepperTrigger>
        </StepperItem>
      ) : null}
      <StepperItem step={3}>
        <StepperTrigger>Third</StepperTrigger>
      </StepperItem>
    </Stepper>
  );
}

describe("Stepper", () => {
  it("Should navigate only across triggers that remain mounted", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<StepperFixture />);
    const first = screen.getByRole("tab", { name: "First" });

    first.focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("tab", { name: "Second" })).toHaveFocus();

    rerender(<StepperFixture includeMiddle={false} />);
    first.focus();
    await user.keyboard("{ArrowRight}");
    expect(screen.getByRole("tab", { name: "Third" })).toHaveFocus();
  });
});
