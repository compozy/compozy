import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentPageActions, AgentPageOverflow, AgentPageStatusPill } from "../agent-page-header";

describe("AgentPageStatusPill", () => {
  it("Should show Active when the exact active count is positive", () => {
    render(<AgentPageStatusPill activeCount={1} />);
    expect(screen.getByTestId("agent-page-status")).toHaveTextContent("Active");
  });

  it("Should show Idle when the exact active count is zero", () => {
    render(<AgentPageStatusPill activeCount={0} />);
    expect(screen.getByTestId("agent-page-status")).toHaveTextContent("Idle");
  });
});

describe("AgentPageActions", () => {
  it("Should expose only the primary New session action", async () => {
    const user = userEvent.setup();
    const onNewSession = vi.fn();

    render(
      <AgentPageActions
        onNewSession={onNewSession}
        isCreatingSession={false}
        newSessionDisabled={false}
      />
    );

    expect(screen.queryByTestId("agent-page-configure")).not.toBeInTheDocument();
    expect(screen.queryByTestId("agent-page-refresh")).not.toBeInTheDocument();
    expect(screen.queryByText("Pause")).not.toBeInTheDocument();
    expect(screen.queryByText("Reset")).not.toBeInTheDocument();
    expect(screen.queryByText("Rename")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "More agent actions" })).not.toBeInTheDocument();

    await user.click(screen.getByTestId("agent-page-new-session"));
    expect(onNewSession).toHaveBeenCalledTimes(1);
  });
});

describe("AgentPageOverflow", () => {
  it("Should expose Edit settings, Duplicate, and Delete in the overflow menu", async () => {
    const user = userEvent.setup();
    const onEditSettings = vi.fn();
    const onDuplicate = vi.fn();
    const onDelete = vi.fn();

    render(
      <AgentPageOverflow
        onDelete={onDelete}
        onDuplicate={onDuplicate}
        onEditSettings={onEditSettings}
      />
    );

    await user.click(screen.getByRole("button", { name: "More agent actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Edit settings" }));
    await user.click(screen.getByRole("button", { name: "More agent actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Duplicate" }));
    await user.click(screen.getByRole("button", { name: "More agent actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete…" }));
    expect(onEditSettings).toHaveBeenCalledTimes(1);
    expect(onDuplicate).toHaveBeenCalledTimes(1);
    expect(onDelete).toHaveBeenCalledTimes(1);
  });
});
