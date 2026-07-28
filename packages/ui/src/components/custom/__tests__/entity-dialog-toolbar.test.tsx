import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { EntityDialogToolbar } from "../entity-dialog-toolbar";

describe("EntityDialogToolbar", () => {
  it("Should render a trailing control with its label", () => {
    render(
      <EntityDialogToolbar
        trailing={<button type="button">launch-hq</button>}
        trailingLabel="Scope"
      />
    );

    expect(screen.getByText("Scope")).toBeInTheDocument();
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
