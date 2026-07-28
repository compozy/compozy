import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

import { UIProvider } from "@compozy/ui";

import { statusKeys } from "@/systems/status";
import { statusFixture } from "@/systems/status/mocks";

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

function renderWorkspaceCommandSelect(children: ReactNode, userHomeDir?: string) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Number.POSITIVE_INFINITY } },
  });
  queryClient.setQueryData(statusKeys.current(), {
    ...statusFixture,
    daemon: {
      ...statusFixture.daemon,
      user_home_dir: userHomeDir ?? statusFixture.daemon.user_home_dir,
    },
  });

  return render(<QueryClientProvider client={queryClient}>{children}</QueryClientProvider>);
}

describe("WorkspaceCommandSelect", () => {
  it("Should render the trigger with the selected workspace name and avatar initial", () => {
    renderWorkspaceCommandSelect(
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

  it("Should render the selected home workspace with the home icon", () => {
    renderWorkspaceCommandSelect(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_beta"
          onChange={() => undefined}
        />
      </UIProvider>,
      "/workspace/beta"
    );

    const avatar = screen.getByTestId("workspace-switcher-avatar");
    expect(avatar).toHaveAttribute("data-home", "true");
    expect(avatar.querySelector("svg")).not.toBeNull();
    expect(avatar).not.toHaveTextContent("B");
    expect(screen.getByTestId("workspace-switcher")).toHaveAccessibleName("Home workspace: beta");
  });

  it("Should align compact trigger height with pill-group md track tokens", () => {
    renderWorkspaceCommandSelect(
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
    renderWorkspaceCommandSelect(
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

    renderWorkspaceCommandSelect(
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

    renderWorkspaceCommandSelect(
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

  it("Should pin the home workspace first in the command list", async () => {
    const user = userEvent.setup();

    renderWorkspaceCommandSelect(
      <UIProvider reducedMotion="never" skipAnimations>
        <WorkspaceCommandSelect
          workspaces={workspaces}
          value="ws_alpha"
          onChange={() => undefined}
        />
      </UIProvider>,
      "/workspace/beta"
    );

    await user.click(screen.getByTestId("workspace-switcher"));

    const group = screen.getByTestId("workspace-command-group");
    const items = Array.from(group.querySelectorAll<HTMLElement>('[data-slot="command-item"]'));
    expect(items.map(item => item.getAttribute("data-testid"))).toEqual([
      "workspace-command-item-ws_beta",
      "workspace-command-item-ws_alpha",
    ]);
    expect(screen.getByTestId("workspace-command-item-ws_beta")).toHaveAttribute(
      "data-home",
      "true"
    );
    expect(screen.getByTestId("workspace-command-item-avatar-ws_beta")).toHaveAttribute(
      "data-home",
      "true"
    );
    expect(screen.getByText("Home workspace")).toBeInTheDocument();
  });

  it("Should filter results via keyboard search", async () => {
    const user = userEvent.setup();

    renderWorkspaceCommandSelect(
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

    renderWorkspaceCommandSelect(
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
});
