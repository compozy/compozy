import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { Field, FieldHeader, FieldLabel } from "../../field";
import { Input } from "../../input";
import { TooltipProvider } from "../../tooltip";
import { HelpTip } from "../help-tip";

function renderTip(ui: React.ReactNode) {
  return render(<TooltipProvider delay={0}>{ui}</TooltipProvider>);
}

describe("HelpTip", () => {
  it("Should expose a named button that keyboard users can reach", async () => {
    const user = userEvent.setup();
    renderTip(<HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>);

    await user.tab();

    expect(screen.getByRole("button", { name: "About category path" })).toHaveFocus();
  });

  it("Should reveal the prose on click so touch users can reach it", async () => {
    const user = userEvent.setup();
    renderTip(<HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>);

    expect(screen.queryByText("Slash-separated catalog grouping.")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "About category path" }));

    expect(screen.getByText("Slash-separated catalog grouping.")).toBeInTheDocument();
  });

  it("Should dismiss the prose on Escape", async () => {
    const user = userEvent.setup();
    renderTip(<HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>);

    await user.click(screen.getByRole("button", { name: "About category path" }));
    expect(screen.getByText("Slash-separated catalog grouping.")).toBeInTheDocument();

    // WCAG 1.4.13: content shown on hover or focus must be dismissible without
    // moving the pointer. Inside a dialog this also has to land before the
    // dialog's own Escape handler.
    await user.keyboard("{Escape}");

    expect(screen.queryByText("Slash-separated catalog grouping.")).not.toBeInTheDocument();
  });

  it("Should keep the trigger out of the label's accessible name", () => {
    renderTip(
      <Field>
        <FieldHeader>
          <FieldLabel htmlFor="agent-name">Agent name</FieldLabel>
          <HelpTip label="About agent name">Lowercase slug sessions launch from.</HelpTip>
        </FieldHeader>
        <Input id="agent-name" />
      </Field>
    );

    // Nesting the trigger inside the `<label>` would fold "About agent name"
    // into the input's name-from-content computation.
    expect(screen.getByLabelText("Agent name")).toBe(screen.getByRole("textbox"));
  });

  it("Should raise the trigger to the touch target below 760px", () => {
    renderTip(<HelpTip label="About category path">Slash-separated catalog grouping.</HelpTip>);

    expect(screen.getByRole("button", { name: "About category path" })).toHaveClass(
      "max-[760px]:size-(--height-button-cta-lg)"
    );
  });
});
