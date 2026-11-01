// Suite: words edited in place on the profile header
// Invariant: Enter or leaving saves, Esc puts the words back, and an invalid value never saves.
// Boundary IN: `AgentInlineText`. Boundary OUT: the update mutation, owned by `use-agents`.
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentInlineText } from "../agent-profile/agent-identity-editor";

function renderName(readOnly = false) {
  const onSave = vi.fn();
  render(
    <AgentInlineText
      label="Name"
      max={10}
      onSave={onSave}
      readOnly={readOnly}
      required
      textClassName=""
      value="Triage"
    />
  );
  return onSave;
}

describe("AgentInlineText", () => {
  it("Should save on Enter", async () => {
    const onSave = renderName();
    await userEvent.click(screen.getByRole("button", { name: "Edit name: Triage" }));
    await userEvent.clear(screen.getByRole("textbox", { name: "Name" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "Lead{Enter}");
    expect(onSave).toHaveBeenCalledWith("Lead");
  });

  it("Should put the words back on Escape", async () => {
    const onSave = renderName();
    await userEvent.click(screen.getByRole("button", { name: "Edit name: Triage" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "x{Escape}");
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Edit name: Triage" })).toBeVisible();
  });

  it("Should hold a value that's too long and say why", async () => {
    const onSave = renderName();
    await userEvent.click(screen.getByRole("button", { name: "Edit name: Triage" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), " is too long{Enter}");
    expect(onSave).not.toHaveBeenCalled();
    expect(screen.getByText("Name can have up to 10 characters.")).toBeVisible();
  });

  it("Should refuse to edit an extension's agent", () => {
    renderName(true);
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
    expect(screen.getByText("Triage")).toBeVisible();
  });
});
