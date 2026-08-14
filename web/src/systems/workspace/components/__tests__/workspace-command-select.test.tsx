import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { emptyWorktreeListingFixture } from "../../mocks";
import type { WorkspacePayload } from "../../types";
import { WorkspaceCommandSelect } from "../workspace-command-select";

function makeWorkspace(
  overrides: Partial<WorkspacePayload> & { id: string; name: string }
): WorkspacePayload {
  return {
    root_dir: `/workspace/${overrides.name}`,
    add_dirs: [],
    created_at: "2026-04-06T10:00:00Z",
    updated_at: "2026-04-06T10:00:00Z",
    ...overrides,
  };
}

const workspaces = [
  makeWorkspace({ id: "ws_alpha", name: "alpha" }),
  makeWorkspace({ id: "ws_beta", name: "beta" }),
];

describe("WorkspaceCommandSelect", () => {
  it("Should render the trigger with the selected workspace name and avatar initial", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>
    );

    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByTestId("workspace-switcher-avatar")).toHaveTextContent("A");
    expect(screen.getByTestId("workspace-switcher-name")).toHaveTextContent("alpha");
    expect(screen.getByTestId("workspace-switcher-chevron")).toBeInTheDocument();
  });

  it("Should associate the trigger with an external field label when provided", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <span id="workspace-field-label">Workspace</span>
        <WorkspaceCommandSelect
          ariaLabelledBy="workspace-field-label"
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>
    );

    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute(
      "aria-labelledby",
      "workspace-field-label"
    );
    expect(screen.getByRole("button", { name: "Workspace" })).toBeInTheDocument();
  });

  it("Should hide the operator-home registration from the list", async () => {
    const user = userEvent.setup();
    const home = makeWorkspace({
      id: "ws_home",
      name: "home",
      root_dir: "/Users/operator",
    });

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          userHomeDir="/Users/operator"
          workspaces={[home, ...workspaces]}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    expect(screen.queryByTestId("workspace-command-item-ws_home")).not.toBeInTheDocument();
    expect(screen.getByTestId("workspace-command-item-ws_alpha")).toBeInTheDocument();
    expect(screen.queryByText("Home workspace")).not.toBeInTheDocument();
  });

  it("Should align compact trigger height with pill-group md track tokens", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
          size="compact"
          triggerTestId="workspace-compact-switcher"
        />
      </UIProvider>
    );

    const trigger = screen.getByTestId("workspace-compact-switcher");
    expect(trigger).toHaveAttribute("data-size", "compact");
    expect(trigger.className).toContain(
      "calc(var(--height-pill-group-segment-md)+2*var(--space-pill-group-track-padding))"
    );
    expect(trigger.className).not.toContain("h-9");
    expect(screen.getByTestId("workspace-switcher-name")).toHaveClass("text-form-label");
  });

  it("Should show No workspace and disable the trigger when the registry is empty", () => {
    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect workspaces={[]} value={null} onChange={() => undefined} />
      </UIProvider>
    );

    expect(screen.getByTestId("workspace-switcher-name")).toHaveTextContent("No workspace");
    expect(screen.getByTestId("workspace-switcher")).toBeDisabled();
  });

  it("Should call onChange with the workspace id and close the popover when an item is selected", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect workspaces={workspaces} value="ws_alpha" onChange={onChange} />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute("aria-expanded", "true");

    await user.click(screen.getByTestId("workspace-command-item-ws_beta"));
    expect(onChange).toHaveBeenCalledWith("ws_beta");
    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute("aria-expanded", "false");
  });

  it("Should mark the active workspace with data-checked in the list", async () => {
    const user = userEvent.setup();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    expect(screen.getByTestId("workspace-command-item-ws_alpha")).toHaveAttribute(
      "data-checked",
      "true"
    );
    expect(screen.getByTestId("workspace-command-item-ws_beta")).toHaveAttribute(
      "data-checked",
      "false"
    );
  });

  it("Should hide the operator-home row and list project workspaces only", async () => {
    const user = userEvent.setup();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          userHomeDir="/workspace/beta"
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));

    const group = screen.getByTestId("workspace-command-group");
    const items = Array.from(group.querySelectorAll<HTMLElement>('[data-slot="command-item"]'));
    expect(items.map(item => item.getAttribute("data-testid"))).toEqual([
      "workspace-command-item-ws_alpha",
    ]);
    expect(screen.queryByText("Home workspace")).not.toBeInTheDocument();
  });

  it("Should filter results via keyboard search", async () => {
    const user = userEvent.setup();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    await user.type(screen.getByTestId("workspace-command-input"), "bet");
    expect(screen.getByTestId("workspace-command-item-ws_beta")).toBeInTheDocument();
    expect(screen.queryByTestId("workspace-command-item-ws_alpha")).not.toBeInTheDocument();
  });

  it("Should call onAddWorkspace when the add action is selected", async () => {
    const user = userEvent.setup();
    const onAddWorkspace = vi.fn();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
          onAddWorkspace={onAddWorkspace}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    await user.click(screen.getByTestId("workspace-command-add"));
    expect(onAddWorkspace).toHaveBeenCalledOnce();
    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute("aria-expanded", "false");
  });

  it("Should select a git-backed workspace and close on a pointer click", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={onChange}
          worktreesByWorkspace={{ [workspaces[0].id]: emptyWorktreeListingFixture }}
          onCreateWorktree={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    const row = screen.getByTestId("workspace-command-item-ws_alpha");
    expect(row).toHaveAttribute("aria-haspopup", "dialog");
    expect(row).toHaveAttribute("aria-expanded", "false");
    expect(
      screen.queryByTestId("workspace-command-worktree-submenu-ws_alpha")
    ).not.toBeInTheDocument();

    await user.click(row);
    expect(onChange).toHaveBeenCalledWith("ws_alpha");
    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute("aria-expanded", "false");
  });

  it("Should open the worktree side submenu on hover without selecting", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={onChange}
          worktreesByWorkspace={{ [workspaces[0].id]: emptyWorktreeListingFixture }}
          onCreateWorktree={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    await user.hover(screen.getByTestId("workspace-command-item-ws_alpha"));

    expect(
      await screen.findByTestId("workspace-command-worktree-submenu-ws_alpha")
    ).toBeInTheDocument();
    expect(screen.getByTestId("workspace-command-worktree-create-ws_alpha")).toBeInTheDocument();
    expect(onChange).not.toHaveBeenCalled();
    expect(screen.getByTestId("workspace-switcher")).toHaveAttribute("aria-expanded", "true");
  });

  it("Should move keyboard focus into the worktree panel and back to its trigger", async () => {
    const user = userEvent.setup();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
          worktreesByWorkspace={{ [workspaces[0].id]: emptyWorktreeListingFixture }}
          onCreateWorktree={() => undefined}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    const input = screen.getByTestId("workspace-command-input");
    await user.keyboard("{ArrowRight}");

    const create = await screen.findByTestId("workspace-command-worktree-create-ws_alpha");
    expect(create).toHaveFocus();
    expect(screen.getByRole("dialog", { name: "Worktrees in alpha" })).toBeInTheDocument();

    await user.keyboard("{ArrowLeft}");
    expect(input).toHaveFocus();
    expect(
      screen.queryByTestId("workspace-command-worktree-submenu-ws_alpha")
    ).not.toBeInTheDocument();
  });

  it("Should leave a non-git workspace as a plain item with no submenu", async () => {
    const user = userEvent.setup();

    render(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
          worktreesByWorkspace={{
            [workspaces[0].id]: emptyWorktreeListingFixture,
            [workspaces[1].id]: {
              worktrees: [],
              discovered: [],
              repo: { git_backed: false, git_available: true },
            },
          }}
        />
      </UIProvider>
    );

    await user.click(screen.getByTestId("workspace-switcher"));
    expect(screen.getByTestId("workspace-command-item-ws_beta")).not.toHaveAttribute(
      "aria-haspopup"
    );
    expect(
      screen.queryByTestId("workspace-command-worktree-submenu-ws_beta")
    ).not.toBeInTheDocument();
  });
});
