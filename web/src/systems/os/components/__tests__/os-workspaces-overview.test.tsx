// Suite: OS workspaces overview (Command-Tab switcher)
// Invariant: one strip layer over one vertical menu layer with roving DOM focus — arrows wrap,
// typeahead cycles, inert rows are skipped, Escape steps one layer at a time, activation lands
// root/scope/create/add with a single close, and current markers follow the scope contract.
// Boundary IN: OsWorkspacesOverview interaction model — tiles, caption, worktree menu, hints.
// Boundary OUT: active-workspace store semantics, worktree dialog flows, transport, and the
// strip's scroll physics (no layout in jsdom).
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { WorkspacePayload, WorktreeListingByWorkspace } from "@/systems/workspace";
import {
  discoveredWorktreeFixture,
  emptyWorktreeListingFixture,
  nonGitWorktreeListingFixture,
  scrollableWorktreesListingFixture,
  worktreeErrorFixture,
  worktreeListingFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
  worktreeSetupFlaggedFixture,
} from "@/systems/workspace/mocks";

import { OsWorkspacesOverview, type OsWorkspacesOverviewProps } from "../os-workspaces-overview";

function workspace(id: string, name: string, rootDir: string): WorkspacePayload {
  return {
    id,
    name,
    root_dir: rootDir,
    add_dirs: [],
    created_at: "2026-04-17T08:00:00Z",
    updated_at: "2026-04-17T17:30:00Z",
  };
}

const COMPOZY = workspace("ws_compozy", "compozy", "/Users/ada/dev/compozy");
const BRANAS = workspace("ws_branas_site", "branas-site", "/Users/ada/dev/branas/site");
const NOTES = workspace("ws_notes", "notes", "/Users/ada/notes");

const LISTINGS: WorktreeListingByWorkspace = {
  [COMPOZY.id]: worktreeListingFixture,
  [BRANAS.id]: emptyWorktreeListingFixture,
  [NOTES.id]: nonGitWorktreeListingFixture,
};

const MIXED_LISTINGS: WorktreeListingByWorkspace = {
  [COMPOZY.id]: {
    worktrees: [
      worktreeSetupFlaggedFixture,
      worktreePendingFixture,
      worktreeMissingFixture,
      worktreeErrorFixture,
    ],
    discovered: [discoveredWorktreeFixture],
    repo: { git_backed: true, git_available: true },
  },
  [BRANAS.id]: emptyWorktreeListingFixture,
  [NOTES.id]: nonGitWorktreeListingFixture,
};

function renderOverview(overrides: Partial<OsWorkspacesOverviewProps> = {}) {
  const callbacks = {
    onOpenChange: vi.fn(),
    onSelectWorkspace: vi.fn(),
    onNewWorkspace: vi.fn(),
    onSelectWorktree: vi.fn(),
    onCreateWorktree: vi.fn(),
    onRemoveWorktree: vi.fn(),
  };
  const props: OsWorkspacesOverviewProps = {
    open: true,
    workspaces: [COMPOZY, BRANAS, NOTES],
    activeWorkspaceId: COMPOZY.id,
    scope: "workspace",
    reducedMotion: true,
    worktreesByWorkspace: LISTINGS,
    userHomeDir: "/Users/ada",
    selectedWorktreeId: null,
    ...callbacks,
    ...overrides,
  };
  render(<OsWorkspacesOverview {...props} />);
  return callbacks;
}

function tile(id: string) {
  return screen.getByTestId(`os-workspace-tile-${id}`);
}

function caption() {
  return screen.getByTestId("os-workspaces-caption");
}

describe("OsWorkspacesOverview", () => {
  it("Should wrap arrow focus across the strip and jump with Home/End", async () => {
    const user = userEvent.setup();
    renderOverview();
    expect(tile(COMPOZY.id)).toHaveFocus();

    // ← from the first tile wraps to the trailing add tile — instantly.
    await user.keyboard("{ArrowLeft}");
    expect(screen.getByTestId("os-workspace-tile-add")).toHaveFocus();
    expect(caption()).toHaveTextContent("New workspace");

    await user.keyboard("{ArrowRight}");
    expect(tile(COMPOZY.id)).toHaveFocus();

    await user.keyboard("{End}");
    expect(screen.getByTestId("os-workspace-tile-add")).toHaveFocus();
    await user.keyboard("{Home}");
    expect(tile(COMPOZY.id)).toHaveFocus();
    // Home-rooted paths display `~`-contracted in the caption.
    expect(caption()).toHaveTextContent("~/dev/compozy");
    expect(caption()).not.toHaveTextContent("/Users/ada");
  });

  it("Should display worktree row paths home-contracted", () => {
    renderOverview();
    const row = screen.getByTestId(
      `os-workspaces-worktree-row-${worktreeReadyDirtyRunningFixture.id}`
    );
    expect(row).toHaveTextContent("~/");
    expect(row).not.toHaveTextContent("/Users/ada");
  });

  it("Should jump by typeahead prefix and cycle a repeated letter through its matches", async () => {
    const user = userEvent.setup();
    renderOverview();

    await user.keyboard("n");
    expect(tile(NOTES.id)).toHaveFocus();

    // "new" matches the add tile: n → notes, repeated n cycles to New.
    await user.keyboard("n");
    expect(screen.getByTestId("os-workspace-tile-add")).toHaveFocus();
  });

  it("Should enter the menu on ↓, leave on ↑ at the top, and step Escape one layer at a time", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.keyboard("{ArrowDown}");
    const firstRow = screen.getByTestId(
      `os-workspaces-worktree-row-${worktreeReadyDirtyRunningFixture.id}`
    );
    expect(firstRow).toHaveFocus();

    await user.keyboard("{ArrowUp}");
    expect(tile(COMPOZY.id)).toHaveFocus();

    await user.keyboard("{ArrowDown}");
    expect(firstRow).toHaveFocus();

    // Escape from the menu returns to the strip without closing the overlay.
    await user.keyboard("{Escape}");
    expect(tile(COMPOZY.id)).toHaveFocus();
    expect(callbacks.onOpenChange).not.toHaveBeenCalledWith(false);

    await user.keyboard("{Escape}");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should close the kebab popover on Escape before leaving the menu layer", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.keyboard("{ArrowDown}");
    const kebab = screen.getByTestId(
      `os-workspaces-worktree-actions-${worktreeReadyDirtyRunningFixture.id}`
    );
    // Shift+F10 on the focused row opens its actions popover (a11y contract).
    await user.keyboard("{Shift>}{F10}{/Shift}");
    await waitFor(() => expect(kebab).toHaveAttribute("aria-expanded", "true"));
    expect(
      await screen.findByTestId(
        `os-workspaces-worktree-delete-${worktreeReadyDirtyRunningFixture.id}`
      )
    ).toBeVisible();

    // First Escape only dismisses the popover; the overlay stays open.
    await user.keyboard("{Escape}");
    await waitFor(() => expect(kebab).toHaveAttribute("aria-expanded", "false"));
    expect(callbacks.onOpenChange).not.toHaveBeenCalledWith(false);

    // The next Escape leaves the menu layer; only the third closes the overlay.
    await user.keyboard("{Escape}");
    expect(tile(COMPOZY.id)).toHaveFocus();
    expect(callbacks.onOpenChange).not.toHaveBeenCalledWith(false);
    await user.keyboard("{Escape}");
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should skip inert rows while their reasons stay readable in the lane", async () => {
    const user = userEvent.setup();
    renderOverview({ worktreesByWorkspace: MIXED_LISTINGS });

    // Inert rows render aria-disabled with a functional reason — never hover-only.
    const pendingRow = screen.getByTestId(
      `os-workspaces-worktree-row-${worktreePendingFixture.id}`
    );
    expect(pendingRow).toHaveAttribute("aria-disabled", "true");
    expect(pendingRow).toHaveTextContent("materializing");
    expect(
      screen.getByTestId(`os-workspaces-worktree-row-${worktreeMissingFixture.id}`)
    ).toHaveTextContent("directory missing");

    await user.keyboard("{ArrowDown}");
    expect(
      screen.getByTestId(`os-workspaces-worktree-row-${worktreeSetupFlaggedFixture.id}`)
    ).toHaveFocus();

    // ↓ skips pending and lands on the discovered (selectable) row.
    await user.keyboard("{ArrowDown}");
    expect(
      screen.getByTestId(`os-workspaces-worktree-row-discovered:${discoveredWorktreeFixture.path}`)
    ).toHaveFocus();

    // ↓ again skips missing + error and reaches the create footer row.
    await user.keyboard("{ArrowDown}");
    expect(screen.getByTestId("os-workspaces-worktree-create")).toHaveFocus();
  });

  it("Should activate a tile to the workspace root and close once", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.keyboard("{ArrowRight}{Enter}");
    expect(callbacks.onSelectWorkspace).toHaveBeenCalledExactlyOnceWith(BRANAS.id);
    expect(callbacks.onOpenChange).toHaveBeenCalledExactlyOnceWith(false);
    expect(callbacks.onSelectWorktree).not.toHaveBeenCalled();
  });

  it("Should scope a worktree row on ↵ and open create from the footer", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.keyboard("{ArrowDown}{Enter}");
    expect(callbacks.onSelectWorktree).toHaveBeenCalledTimes(1);
    const [workspaceId, entry] = callbacks.onSelectWorktree.mock.calls[0] ?? [];
    expect(workspaceId).toBe(COMPOZY.id);
    expect(entry?.key).toBe(worktreeReadyDirtyRunningFixture.id);
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
    expect(callbacks.onSelectWorkspace).not.toHaveBeenCalled();
  });

  it("Should open the create flow from the menu footer", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview();

    await user.keyboard("{ArrowDown}{End}{Enter}");
    expect(callbacks.onCreateWorktree).toHaveBeenCalledExactlyOnceWith(COMPOZY.id);
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should keep picking live while Global is on — the store owns turning it off", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({ scope: "global" });

    expect(screen.getByTestId("os-workspaces-notice")).toHaveTextContent(
      "Global scope is on — picking a workspace turns it off."
    );
    await user.keyboard("{Enter}");
    expect(callbacks.onSelectWorkspace).toHaveBeenCalledExactlyOnceWith(COMPOZY.id);
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should mark current as check, flip to the branch badge under a worktree scope, and hide all under Global", () => {
    renderOverview();
    expect(tile(COMPOZY.id)).toHaveAttribute("data-current", "root");
  });

  it("Should move the check into the menu row while the tile carries the branch badge", () => {
    renderOverview({ selectedWorktreeId: worktreeReadyDirtyRunningFixture.id });
    expect(tile(COMPOZY.id)).toHaveAttribute("data-current", "wt");
    expect(
      screen.getByTestId(`os-workspaces-worktree-row-${worktreeReadyDirtyRunningFixture.id}`)
    ).toHaveAttribute("aria-selected", "true");
  });

  it("Should show no current marker anywhere while Global is on", () => {
    renderOverview({ scope: "global", selectedWorktreeId: null });
    expect(tile(COMPOZY.id)).not.toHaveAttribute("data-current");
    expect(tile(BRANAS.id)).not.toHaveAttribute("data-current");
  });

  it("Should render menu absence truthfully: git+0 → lone create button, non-git → nothing", async () => {
    const user = userEvent.setup();
    renderOverview();

    // branas-site is git-backed with zero worktrees: only the dashed button.
    await user.keyboard("{ArrowRight}");
    expect(screen.getByTestId("os-workspaces-worktree-new")).toBeInTheDocument();
    expect(screen.queryByTestId("os-workspaces-worktree-menu")).not.toBeInTheDocument();

    // notes is not git-backed: the affordance is absent, never disabled.
    await user.keyboard("{ArrowRight}");
    expect(screen.queryByTestId("os-workspaces-worktree-new")).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-workspaces-worktree-menu")).not.toBeInTheDocument();
  });

  it("Should truncate the nest at five rows and keep only real destinations in the footer", () => {
    renderOverview({
      worktreesByWorkspace: {
        [COMPOZY.id]: scrollableWorktreesListingFixture,
        [BRANAS.id]: emptyWorktreeListingFixture,
        [NOTES.id]: nonGitWorktreeListingFixture,
      },
    });

    const menu = screen.getByTestId("os-workspaces-worktree-menu");
    expect(menu.querySelectorAll('[data-slot="os-workspaces-worktree-row"]')).toHaveLength(5);
    expect(screen.getByTestId("os-workspaces-worktree-create")).toBeInTheDocument();
    // No "All N worktrees" row: production has no worktree-overview surface,
    // and this overlay must not invent a destination.
    expect(screen.queryByText(/^All \d+ worktrees$/)).not.toBeInTheDocument();
  });

  it("Should render the one primary and the Global identity at zero workspaces", async () => {
    const user = userEvent.setup();
    const callbacks = renderOverview({
      workspaces: [],
      worktreesByWorkspace: {},
      activeWorkspaceId: null,
      scope: "global",
    });

    expect(
      screen.getByText("No project folders yet. Sessions run in Global until you add one.")
    ).toBeInTheDocument();
    expect(caption()).toHaveTextContent("Global");
    expect(caption()).toHaveTextContent("visible to every workspace");
    expect(screen.getByTestId("os-workspaces-subtitle")).toHaveTextContent("0 workspaces");
    // The empty state has no strip hints to promise — and no notice line.
    expect(screen.queryByTestId("os-workspaces-notice")).not.toBeInTheDocument();

    await user.click(screen.getByTestId("os-workspaces-new"));
    expect(callbacks.onNewWorkspace).toHaveBeenCalledTimes(1);
    expect(callbacks.onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should count adopted worktrees only and singularize one workspace", () => {
    renderOverview({
      workspaces: [COMPOZY],
      worktreesByWorkspace: { [COMPOZY.id]: worktreeListingFixture },
    });
    // 7 adopted records — the discovered checkout never enters the count.
    expect(screen.getByTestId("os-workspaces-subtitle")).toHaveTextContent(
      "1 workspace · 7 worktrees"
    );
  });
});
