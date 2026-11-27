// Suite: session environment field
// Invariant (UT-132): the trigger truthfully names root, ready, new, and missing targets while
// only ready worktrees are offered and its label/error are associated with the trigger.
// A selected worktree displays one dropdown chevron.
// Owning layer: session-create environment component. Canonical suite: this file.
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { buildWorktreeFixture } from "@/systems/workspace/mocks/worktree-fixtures";

import { SessionEnvironmentField } from "../session-environment-field";

const materialization = {
  status: "idle" as const,
  start: vi.fn(),
  cancel: vi.fn(),
  recover: vi.fn(),
  retry: vi.fn(),
  reset: vi.fn(),
};

describe("SessionEnvironmentField", () => {
  it("Should show one dropdown chevron for a selected worktree", () => {
    render(
      <SessionEnvironmentField
        onChange={vi.fn()}
        value={{ kind: "worktree", worktreeId: "wt_ready" }}
        worktrees={[buildWorktreeFixture({ id: "wt_ready", name: "checkout", state: "ready" })]}
      />
    );

    const trigger = screen.getByRole("button", { name: "Environment" });
    expect(trigger).toHaveTextContent("checkout");
    expect(trigger.querySelectorAll("svg[aria-hidden='true']")).toHaveLength(1);
  });

  it("Should show New worktree as the selected intent and keep only ready rows selectable", () => {
    const onChange = vi.fn();
    render(
      <SessionEnvironmentField
        materialization={materialization}
        onChange={onChange}
        value={{ kind: "new", name: "", previous: { kind: "root" } }}
        worktrees={[
          buildWorktreeFixture({ id: "wt_ready", name: "checkout", state: "ready" }),
          buildWorktreeFixture({ id: "wt_missing", name: "removed", state: "missing" }),
        ]}
      />
    );

    const trigger = screen.getByTestId("session-create-environment");
    expect(trigger).toHaveTextContent("New worktree…");
    expect(screen.getByText("Environment")).toHaveAttribute("for", "session-environment");

    fireEvent.click(trigger);
    expect(screen.getByRole("option", { name: /checkout/ })).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: /removed/ })).not.toBeInTheDocument();
  });
});
