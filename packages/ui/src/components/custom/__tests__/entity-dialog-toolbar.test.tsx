import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { EntityDialogToolbar } from "../entity-dialog-toolbar";

describe("EntityDialogToolbar", () => {
  it("Should not paint a chrome strip without a mode control", () => {
    const { container } = render(<EntityDialogToolbar trailing={<span>status</span>} />);

    expect(container.querySelector('[data-slot="entity-dialog-toolbar"]')).not.toHaveClass(
      "bg-canvas-tint"
    );
  });

  it("Should render its trailing control", () => {
    render(<EntityDialogToolbar trailing={<button type="button">launch-hq</button>} />);

    expect(screen.getByRole("button", { name: "launch-hq" })).toBeInTheDocument();
  });

  it("Should keep its sole trailing control reachable", async () => {
    const user = userEvent.setup();
    render(<EntityDialogToolbar trailing={<button type="button">launch-hq</button>} />);

    await user.tab();
    expect(screen.getByRole("button", { name: "launch-hq" })).toHaveFocus();
  });

  it("Should preserve leading-to-trailing tab order", async () => {
    const user = userEvent.setup();
    render(
      <EntityDialogToolbar
        leading={<button type="button">Simple</button>}
        trailing={<button type="button">launch-hq</button>}
      />
    );

    await user.tab();
    expect(screen.getByRole("button", { name: "Simple" })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole("button", { name: "launch-hq" })).toHaveFocus();
  });
});
