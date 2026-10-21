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

import type { WorkspacePayload, WorktreesResponse } from "@/systems/workspace";
import {
  discoveredOnlyWorktreeListingFixture,
  discoveredWorktreeFixture,
  emptyWorktreeListingFixture,
  nonGitWorktreeListingFixture,
  worktreeListingFixture,
  worktreeBehindFixture,
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import { CmdPaletteRegistryProvider } from "../../../contexts/cmd-palette-registry-context";
import {
  paletteRegistryFixture,
  resolvedPaletteCommand,
} from "../../../mocks/cmd-palette-fixtures";
import { WorkspaceMenu } from "../workspace-menu";

const WORKSPACE_REGISTRY = paletteRegistryFixture([
  resolvedPaletteCommand({
    id: "workspace.picker",
    title: "Workspace picker",
    availability_exempt: true,
  }),
]);

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

/** Listed rows belong to the workspace that lists them, as the daemon guarantees. */
function listedIn(workspaceId: string, listing: WorktreesResponse): WorktreesResponse {
  return {
    ...listing,
    worktrees: listing.worktrees.map(row => ({ ...row, workspace_id: workspaceId })),
  };
}
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
      <CmdPaletteRegistryProvider registry={WORKSPACE_REGISTRY}>
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
              [gitAlpha.id]: listedIn(gitAlpha.id, worktreeListingFixture),
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
      </CmdPaletteRegistryProvider>
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
  it("Should select missing records without navigation and freeze the represented eligible population", async () => {
    const user = userEvent.setup();
    const onRemoveWorktrees = vi.fn();
    const missing = { ...worktreeMissingFixture, workspace_id: gitAlpha.id };
    const callbacks = renderMenu({
      onRemoveWorktrees,
      worktreesByWorkspace: {
        [gitAlpha.id]: {
          worktrees: [missing],
          discovered: [discoveredWorktreeFixture],
          repo: { git_backed: true, git_available: true },
        },
      },
    });
    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await user.click(screen.getByText("Select worktrees…"));
    const checkbox = screen.getByRole("menuitemcheckbox", { name: `Select ${missing.name}` });
    await user.click(checkbox);
    expect(checkbox).toHaveAttribute("aria-checked", "true");
    expect(callbacks.onSelectWorktree).not.toHaveBeenCalled();
    expect(openChangeFlags(callbacks.onOpenChange)).not.toContain(false);
    await user.click(screen.getByText("Remove 1 worktree…"));
    expect(
      onRemoveWorktrees.mock.calls[0]?.[0].worktrees.map((row: { id: string }) => row.id)
    ).toEqual([missing.id]);
  });

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

  it("Should dot the monogram of every workspace with working sessions", () => {
    renderMenu({ workingSessionsByWorkspace: { [gitBeta.id]: 2, [plainNotes.id]: 1 } });

    expect(
      within(screen.getByTestId(`os-workspace-option-${gitBeta.id}`)).getByRole("img", {
        name: "2 sessions working",
      })
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId(`os-workspace-option-${plainNotes.id}`)).getByRole("img", {
        name: "1 session working",
      })
    ).toBeInTheDocument();
    expect(
      within(screen.getByTestId(`os-workspace-option-${gitAlpha.id}`)).queryByRole("img", {
        name: /working/,
      })
    ).not.toBeInTheDocument();
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
    const busy = screen.getByTestId("os-worktree-remove-wt_payments_retry");
    expect(busy).toHaveAttribute("data-disabled");
    expect(busy).toHaveTextContent("Worktree has an active session");

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, `os-worktree-actions-${worktreeBehindFixture.id}`);
    act(() => screen.getByTestId(`os-worktree-remove-${worktreeBehindFixture.id}`).focus());
    await user.keyboard("{Enter}");

    expect(onRemoveWorktree).toHaveBeenCalledTimes(1);
    const [workspaceId, entry] = onRemoveWorktree.mock.calls[0];
    expect(workspaceId).toBe(gitAlpha.id);
    expect(entry.key).toBe(worktreeBehindFixture.id);
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
        [gitAlpha.id]: listedIn(gitAlpha.id, {
          ...worktreeListingFixture,
          worktrees: [worktreeMissingFixture],
          discovered: [],
        }),
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

  // Regression #813: cleanup follows the row owner, and a refused cleanup stays
  // visible as a disabled item naming its reason instead of disappearing.
  it("Should keep cleanup visible for every owner and name refusals on disabled items", async () => {
    const user = userEvent.setup();
    const foreign = {
      ...worktreeMissingFixture,
      workspace_id: gitAlpha.id,
      profile_id: "profile-design",
      profile_name: "open-design",
    };
    const orphan = {
      ...worktreeMissingFixture,
      id: "wt_orphan_missing",
      name: "orphan-missing",
      workspace_id: gitAlpha.id,
      profile_id: "profile-deleted",
      profile_name: "",
    };
    const { onResolveMissingWorktree } = renderMenu({
      onRemoveWorktrees: vi.fn(),
      worktreesByWorkspace: {
        [gitAlpha.id]: { ...worktreeListingFixture, worktrees: [foreign, orphan], discovered: [] },
        [gitBeta.id]: emptyWorktreeListingFixture,
        [plainNotes.id]: nonGitWorktreeListingFixture,
      },
    });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, `os-worktree-actions-${foreign.id}`);
    act(() => screen.getByTestId(`os-worktree-resolve-${foreign.id}`).focus());
    await user.keyboard("{Enter}");
    expect(onResolveMissingWorktree.mock.calls[0]?.[1]?.key).toBe(foreign.id);

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, `os-worktree-actions-${orphan.id}`);
    const refused = await screen.findByTestId(`os-worktree-resolve-${orphan.id}`);
    expect(refused).toHaveAttribute("data-disabled");
    expect(refused).toHaveTextContent("Owning profile is unavailable");
  });

  it("Should offer Remove on a discovered row and route it without adopting", async () => {
    const user = userEvent.setup();
    const { onRemoveWorktree } = renderMenu({
      worktreesByWorkspace: {
        [gitAlpha.id]: discoveredOnlyWorktreeListingFixture,
        [gitBeta.id]: emptyWorktreeListingFixture,
        [plainNotes.id]: nonGitWorktreeListingFixture,
      },
    });

    await openSubmenuByKeyboard(user, `os-workspace-option-${gitAlpha.id}`);
    await openSubmenuByKeyboard(user, `os-worktree-actions-${DISCOVERED_KEY}`);
    act(() => screen.getByTestId(`os-worktree-remove-${DISCOVERED_KEY}`).focus());
    await user.keyboard("{Enter}");

    expect(onRemoveWorktree).toHaveBeenCalledTimes(1);
    expect(onRemoveWorktree.mock.calls[0]?.[0]).toBe(gitAlpha.id);
    expect(onRemoveWorktree.mock.calls[0]?.[1]?.discovered?.path).toBeTruthy();
  });

  it("Should omit info and warning notices from the menu", () => {
    renderMenu({ globalScopeOn: true });

    expect(screen.queryByText(/Global scope is on/)).not.toBeInTheDocument();
    expect(screen.queryByText(/remembered workspace was removed/)).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-workspace-deletion-notice")).not.toBeInTheDocument();
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("Should run workspace.picker from the registry row", async () => {
    const user = userEvent.setup();
    const { onRun } = renderMenu();

    await user.click(screen.getByTestId("os-menubar-command-workspace.picker"));

    expect(onRun).toHaveBeenCalledExactlyOnceWith("workspace.picker");
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
