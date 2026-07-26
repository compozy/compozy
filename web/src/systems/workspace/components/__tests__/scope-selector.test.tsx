import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { UIProvider } from "@agh/ui";

import { resetUserHomeDirStore, useUserHomeDirStore } from "../../hooks/use-user-home-dir-store";
import { ScopeSelector } from "../scope-selector";

const workspaces = [
  { id: "ws_alpha", name: "alpha", root_dir: "/workspace/alpha" },
  { id: "ws_beta", name: "beta", root_dir: "/workspace/beta" },
];

function renderScopeSelector(props: Partial<ComponentProps<typeof ScopeSelector>> = {}) {
  const onScopeChange = vi.fn();
  const onWorkspaceChange = vi.fn();

  render(
    <UIProvider reducedMotion="always">
      <ScopeSelector
        scope="workspace"
        workspaceId="ws_alpha"
        workspaces={workspaces}
        onScopeChange={onScopeChange}
        onWorkspaceChange={onWorkspaceChange}
        testIdPrefix="task"
        {...props}
      />
    </UIProvider>
  );

  return { onScopeChange, onWorkspaceChange };
}

describe("ScopeSelector", () => {
  beforeEach(() => {
    resetUserHomeDirStore();
  });

  afterEach(() => {
    resetUserHomeDirStore();
  });

  it("Should request global scope from workspace mode", async () => {
    const user = userEvent.setup();
    const { onScopeChange } = renderScopeSelector();

    await user.click(screen.getByTestId("task-scope-global"));
    expect(onScopeChange).toHaveBeenCalledWith("global");
  });

  it("Should request workspace scope from global mode", async () => {
    const user = userEvent.setup();
    const { onScopeChange } = renderScopeSelector({ scope: "global", workspaceId: null });

    await user.click(screen.getByTestId("task-scope-workspace"));
    expect(onScopeChange).toHaveBeenCalledWith("workspace");
  });

  it("Should select a workspace through the command selector", async () => {
    const user = userEvent.setup();
    const { onWorkspaceChange } = renderScopeSelector();

    await user.click(screen.getByTestId("task-workspace-select"));
    await user.click(screen.getByTestId("task-workspace-item-ws_beta"));

    expect(onWorkspaceChange).toHaveBeenCalledWith("ws_beta");
  });

  it("Should promote scope to global when the home workspace is selected", async () => {
    const user = userEvent.setup();
    useUserHomeDirStore.getState().setUserHomeDir("/workspace/beta");
    const { onScopeChange, onWorkspaceChange } = renderScopeSelector();

    await user.click(screen.getByTestId("task-workspace-select"));
    await user.click(screen.getByTestId("task-workspace-item-ws_beta"));

    expect(onScopeChange).toHaveBeenCalledWith("global");
    expect(onWorkspaceChange).not.toHaveBeenCalled();
  });

  it("Should keep workspace scope when a non-home workspace is selected", async () => {
    const user = userEvent.setup();
    useUserHomeDirStore.getState().setUserHomeDir("/workspace/beta");
    const { onScopeChange, onWorkspaceChange } = renderScopeSelector();

    await user.click(screen.getByTestId("task-workspace-select"));
    await user.click(screen.getByTestId("task-workspace-item-ws_alpha"));

    expect(onWorkspaceChange).toHaveBeenCalledWith("ws_alpha");
    expect(onScopeChange).not.toHaveBeenCalled();
  });

  it("Should omit the workspace command selector while global scope is active", () => {
    renderScopeSelector({ scope: "global", workspaceId: null });

    expect(screen.getByTestId("task-scope-global")).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByTestId("task-workspace-select")).not.toBeInTheDocument();
  });

  it("Should disable workspace scope when the parent surface owns scope forcing", () => {
    renderScopeSelector({ scope: "global", workspaceDisabled: true });

    expect(screen.getByTestId("task-scope-workspace")).toBeDisabled();
    expect(screen.queryByTestId("task-workspace-select")).not.toBeInTheDocument();
  });

  it("Should pin the home workspace first and mark it in the compact command selector", async () => {
    const user = userEvent.setup();
    useUserHomeDirStore.getState().setUserHomeDir("/workspace/beta");

    renderScopeSelector();

    await user.click(screen.getByTestId("task-workspace-select"));

    const group = screen.getByTestId("task-workspace-group");
    const items = Array.from(group.querySelectorAll<HTMLElement>('[data-slot="command-item"]'));
    expect(items.map(item => item.getAttribute("data-testid"))).toEqual([
      "task-workspace-item-ws_beta",
      "task-workspace-item-ws_alpha",
    ]);
    expect(screen.getByTestId("task-workspace-item-ws_beta")).toHaveAttribute("data-home", "true");
    expect(screen.getByText("Home workspace")).toBeInTheDocument();
  });
});
