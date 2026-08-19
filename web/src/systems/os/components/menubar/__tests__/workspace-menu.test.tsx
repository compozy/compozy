// Suite: WorkspaceMenu worktree submenu
// Invariant: a git-backed workspace row is a submenu trigger — a pointer click
// selects the workspace and closes the menu, while keyboard traversal opens the
// side submenu holding the root row, the worktree rows, and New worktree. A
// non-git workspace stays a plain selectable item with no submenu affordance.
// Owning layer: WorkspaceMenu. No existing suite owned this fold.
import type { ComponentProps } from "react";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { Menubar, MenubarTrigger, UIProvider } from "@compozy/ui";

import type { WorkspacePayload } from "@/systems/workspace";
import {
  discoveredOnlyWorktreeListingFixture,
  discoveredWorktreeFixture,
  emptyWorktreeListingFixture,
  nonGitWorktreeListingFixture,
  worktreeListingFixture,
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { WorkspaceMenu } from "../workspace-menu";

function makeWorkspace(id: string, name: string): WorkspacePayload {
  return {
    id,
    name,
    root_dir: `/workspace/${name}`,
    add_dirs: [],
    created_at: "2026-04-06T10:00:00Z",
    updated_at: "2026-04-06T10:00:00Z",
  };
}

const gitAlpha = makeWorkspace("ws_alpha", "alpha");
const gitBeta = makeWorkspace("ws_beta", "beta");
const plainNotes = makeWorkspace("ws_notes", "notes");

const DISCOVERED_KEY = `discovered:${discoveredWorktreeFixture.path}`;

function renderMenu(overrides: Partial<ComponentProps<typeof WorkspaceMenu>> = {}) {
  const onOpenChange = vi.fn();
  const onSelectWorkspace = vi.fn();
  const onSelectWorktree = vi.fn();
  const onCreateWorktree = vi.fn();
  const onResolveMissingWorktree = vi.fn();
  const onOpenWorktreeContext = vi.fn();
  const onRemoveWorktree = vi.fn();
  const onRun = vi.fn();
  render(
    <UIProvider reducedMotion="never" skipAnimations>
      <Menubar>
        <WorkspaceMenu
          trigger={<MenubarTrigger data-testid="workspace-trigger">Workspaces</MenubarTrigger>}
          open
          onOpenChange={onOpenChange}
          workspaces={[gitAlpha, gitBeta, plainNotes]}
          activeWorkspaceId={gitAlpha.id}
          monogram={name => name.slice(0, 2).toUpperCase()}
          onSelectWorkspace={onSelectWorkspace}
          onRun={onRun}
          onAddWorkspace={vi.fn()}
          worktreesByWorkspace={{
            [gitAlpha.id]: worktreeListingFixture,
            [gitBeta.id]: emptyWorktreeListingFixture,
            [plainNotes.id]: nonGitWorktreeListingFixture,
          }}
          onSelectWorktree={onSelectWorktree}
          onCreateWorktree={onCreateWorktree}
          onResolveMissingWorktree={onResolveMissingWorktree}
          onOpenWorktreeContext={onOpenWorktreeContext}
          onRemoveWorktree={onRemoveWorktree}
          {...overrides}
        />
      </Menubar>
    </UIProvider>
  );
  return {
    onOpenChange,
    onSelectWorkspace,
    onSelectWorktree,
    onCreateWorktree,
    onResolveMissingWorktree,
    onOpenWorktreeContext,
    onRemoveWorktree,
    onRun,
  };
}

/** Keyboard path into a submenu: highlight the trigger row, then ArrowRight. */
async function openSubmenuByKeyboard(user: ReturnType<typeof userEvent.setup>, testId: string) {
  const trigger = screen.getByTestId(testId);
  act(() => trigger.focus());
  await user.keyboard("{ArrowRight}");
}

/** Base UI calls `onOpenChange(open, eventDetails)`; only the flag matters here. */
function openChangeFlags(onOpenChange: ReturnType<typeof vi.fn>): boolean[] {
  return onOpenChange.mock.calls.map(call => call[0] as boolean);
}

describe("WorkspaceMenu", () => {
  it("Should render git-backed workspaces as submenu triggers and non-git ones as plain items", () => {
    renderMenu();

    expect(screen.getByTestId(`os-workspace-option-${gitAlpha.id}`)).toHaveAttribute(
      "aria-haspopup",
      "menu"
    );
    expect(screen.getByTestId(`os-workspace-option-${plainNotes.id}`)).not.toHaveAttribute(
      "aria-haspopup"
    );
    // No submenu content mounts until the trigger is traversed.
    expect(screen.queryByTestId(`os-worktree-submenu-${gitAlpha.id}`)).not.toBeInTheDocument();
    expect(screen.queryByTestId(`os-worktree-create-${gitAlpha.id}`)).not.toBeInTheDocument();
  });

  it("Should select the workspace and close the menu on a pointer click", async () => {
    const user = userEvent.setup();
    const { onOpenChange, onSelectWorkspace } = renderMenu();

    await user.click(screen.getByTestId(`os-workspace-option-${gitAlpha.id}`));

    expect(onSelectWorkspace).toHaveBeenCalledWith(gitAlpha.id);
    expect(openChangeFlags(onOpenChange)).toContain(false);
  });

  it("Should open the worktree submenu from the keyboard without selecting anything", async () => {
    const user = userEvent.setup();
    const { onOpenChange, onSelectWorkspace, onRun } = renderMenu();

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);

    const submenu = await screen.findByTestId(`os-worktree-submenu-${gitAlpha.id}`);
    const row = screen.getByTestId("os-worktree-option-wt_payments_retry");
    // Nothing is bound yet, so no row is checked.
    expect(row).toHaveAttribute("aria-checked", "false");
    // Every worktree of the workspace stays in the list — adopted and
    // discovered alike — inside the nest's scroll region; long nests scroll
    // instead of folding rows behind an overflow jump to the overview.
    const options = within(submenu).getAllByTestId(/^os-worktree-option-/);
    expect(options).toHaveLength(
      worktreeListingFixture.worktrees.length + worktreeListingFixture.discovered.length
    );
    expect(within(submenu).queryByTestId(/os-worktree-overflow/)).not.toBeInTheDocument();
    expect(row.closest('[data-slot="scroll-area"]')).not.toBeNull();
    // The create row stays reachable below the scroll region, never inside it.
    const create = screen.getByTestId(`os-worktree-create-${gitAlpha.id}`);
    expect(create.closest('[data-slot="scroll-area"]')).toBeNull();
    // Traversal alone neither selects nor closes.
    expect(onSelectWorkspace).not.toHaveBeenCalled();
    expect(openChangeFlags(onOpenChange)).not.toContain(false);
    expect(onRun).not.toHaveBeenCalled();
  });

  it("Should contract home-rooted nest paths when the host supplies userHomeDir", async () => {
    const user = userEvent.setup();
    renderMenu({ userHomeDir: "/Users/ada" });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);

    const row = await screen.findByTestId("os-worktree-option-wt_payments_retry");
    expect(within(row).getByText("~/.compozy/worktrees/launch-hq/payments-retry")).toBeVisible();
  });

  it("Should check the bound worktree, select rows, and close the menu on selection", async () => {
    const user = userEvent.setup();
    const { onOpenChange, onSelectWorktree } = renderMenu({
      selectedWorktreeId: "wt_payments_retry",
    });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);

    const row = await screen.findByTestId("os-worktree-option-wt_payments_retry");
    expect(row).toHaveAttribute("aria-checked", "true");

    await user.click(row);

    expect(onSelectWorktree).toHaveBeenCalledTimes(1);
    const [workspaceId, entry] = onSelectWorktree.mock.calls[0];
    expect(workspaceId).toBe(gitAlpha.id);
    expect(entry.key).toBe("wt_payments_retry");
    expect(openChangeFlags(onOpenChange)).toContain(false);
  });

  it("Should keep a discovered row adoptable with its Adopt affordance announced", async () => {
    const user = userEvent.setup();
    const { onSelectWorktree } = renderMenu({
      worktreesByWorkspace: {
        [gitAlpha.id]: discoveredOnlyWorktreeListingFixture,
        [gitBeta.id]: emptyWorktreeListingFixture,
        [plainNotes.id]: nonGitWorktreeListingFixture,
      },
    });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);

    const row = await screen.findByTestId(`os-worktree-option-${DISCOVERED_KEY}`);
    expect(row).not.toHaveAttribute("aria-disabled", "true");
    expect(within(row).getByText("Adopt")).toBeInTheDocument();

    await user.click(row);

    expect(onSelectWorktree).toHaveBeenCalledTimes(1);
    const [, entry] = onSelectWorktree.mock.calls[0];
    expect(entry.kind).toBe("discovered");
  });

  it("Should expose Copy path and Remove behind a ready row's actions", async () => {
    const user = userEvent.setup();
    const { onRemoveWorktree } = renderMenu();

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, "os-worktree-actions-wt_payments_retry");

    act(() => screen.getByTestId("os-worktree-copy-path-wt_payments_retry").focus());
    await user.keyboard("{Enter}");
    expect(await navigator.clipboard.readText()).toBe(worktreeReadyDirtyRunningFixture.path);

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, "os-worktree-actions-wt_payments_retry");
    act(() => screen.getByTestId("os-worktree-remove-wt_payments_retry").focus());
    await user.keyboard("{Enter}");

    expect(onRemoveWorktree).toHaveBeenCalledTimes(1);
    const [workspaceId, entry] = onRemoveWorktree.mock.calls[0];
    expect(workspaceId).toBe(gitAlpha.id);
    expect(entry.key).toBe("wt_payments_retry");
  });

  it("Should expose Context behind a ready row's actions", async () => {
    const user = userEvent.setup();
    const { onOpenWorktreeContext } = renderMenu();

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, "os-worktree-actions-wt_payments_retry");
    act(() => screen.getByTestId("os-worktree-context-wt_payments_retry").focus());
    await user.keyboard("{Enter}");

    expect(onOpenWorktreeContext).toHaveBeenCalledTimes(1);
    expect(onOpenWorktreeContext.mock.calls[0]?.[0]).toBe(gitAlpha.id);
    expect(onOpenWorktreeContext.mock.calls[0]?.[1]?.key).toBe("wt_payments_retry");
  });

  it("Should expose Resolve behind a missing row's actions", async () => {
    const user = userEvent.setup();
    const { onResolveMissingWorktree } = renderMenu({
      worktreesByWorkspace: {
        [gitAlpha.id]: {
          ...worktreeListingFixture,
          worktrees: [worktreeMissingFixture],
          discovered: [],
        },
        [gitBeta.id]: emptyWorktreeListingFixture,
        [plainNotes.id]: nonGitWorktreeListingFixture,
      },
    });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, `os-worktree-actions-${worktreeMissingFixture.id}`);
    act(() => screen.getByTestId(`os-worktree-resolve-${worktreeMissingFixture.id}`).focus());
    await user.keyboard("{Enter}");

    expect(onResolveMissingWorktree).toHaveBeenCalledTimes(1);
    expect(onResolveMissingWorktree.mock.calls[0]?.[0]).toBe(gitAlpha.id);
    expect(onResolveMissingWorktree.mock.calls[0]?.[1]?.key).toBe(worktreeMissingFixture.id);
  });

  it("Should omit Remove from a discovered row's actions", async () => {
    const user = userEvent.setup();
    renderMenu({
      worktreesByWorkspace: {
        [gitAlpha.id]: discoveredOnlyWorktreeListingFixture,
        [gitBeta.id]: emptyWorktreeListingFixture,
        [plainNotes.id]: nonGitWorktreeListingFixture,
      },
    });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, `os-worktree-actions-${DISCOVERED_KEY}`);

    await screen.findByTestId(`os-worktree-copy-path-${DISCOVERED_KEY}`);
    expect(screen.queryByTestId(`os-worktree-remove-${DISCOVERED_KEY}`)).not.toBeInTheDocument();
  });

  it("Should omit info and warning notices from the menu", () => {
    renderMenu({ globalScopeOn: true });

    expect(screen.queryByText(/Global scope is on/)).not.toBeInTheDocument();
    expect(screen.queryByText(/remembered workspace was removed/)).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-workspace-deletion-notice")).not.toBeInTheDocument();
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("Should omit Remove while a worktree is not ready", async () => {
    const user = userEvent.setup();
    renderMenu();

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, "os-worktree-actions-wt_docs_refresh");

    await screen.findByTestId("os-worktree-copy-path-wt_docs_refresh");
    expect(screen.queryByTestId("os-worktree-remove-wt_docs_refresh")).not.toBeInTheDocument();
  });
});
