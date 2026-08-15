import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import type { WorkspacePayload, WorktreeListingByWorkspace } from "@/systems/workspace";
import {
  discoveredBrokenWorktreeFixture,
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

const THREE_WORKSPACES = [COMPOZY, BRANAS, NOTES];

/** compozy carries the canonical nest; branas-site is git+0; notes is non-git. */
const THREE_LISTINGS: WorktreeListingByWorkspace = {
  [COMPOZY.id]: worktreeListingFixture,
  [BRANAS.id]: emptyWorktreeListingFixture,
  [NOTES.id]: nonGitWorktreeListingFixture,
};

/**
 * Non-ready vocabulary: one flagged ready, then pending/discovered/missing/
 * error, plus a broken checkout whose long git reason truncates in its lane.
 */
const MIXED_LISTINGS: WorktreeListingByWorkspace = {
  [COMPOZY.id]: {
    worktrees: [
      worktreeSetupFlaggedFixture,
      worktreePendingFixture,
      worktreeMissingFixture,
      worktreeErrorFixture,
    ],
    discovered: [discoveredWorktreeFixture, discoveredBrokenWorktreeFixture],
    repo: { git_backed: true, git_available: true },
  },
  [BRANAS.id]: emptyWorktreeListingFixture,
  [NOTES.id]: nonGitWorktreeListingFixture,
};

const MANY_WORKSPACES = [
  COMPOZY,
  BRANAS,
  NOTES,
  workspace("ws_agh_docs", "agh-docs", "/Users/ada/dev/agh/docs"),
  workspace("ws_payments", "payments", "/Users/ada/dev/payments"),
  workspace("ws_infra", "infra", "/Users/ada/dev/infra"),
  workspace("ws_design", "design-system", "/Users/ada/dev/compozy/packages/ui"),
  workspace("ws_sandbox", "sandbox", "/Users/ada/dev/sandbox"),
  workspace("ws_branas_ia", "branas-ia", "/Users/ada/dev/branas/ia"),
  workspace("ws_ext", "compozy-ext", "/Users/ada/dev/compozy-extensions"),
  workspace("ws_site", "site", "/Users/ada/dev/compozy/packages/site"),
  workspace("ws_cli", "cli", "/Users/ada/dev/compozy/cmd"),
];

function baseProps(): OsWorkspacesOverviewProps {
  return {
    open: true,
    onOpenChange: fn(),
    workspaces: THREE_WORKSPACES,
    activeWorkspaceId: COMPOZY.id,
    scope: "workspace",
    onSelectWorkspace: fn(),
    onNewWorkspace: fn(),
    reducedMotion: true,
    worktreesByWorkspace: THREE_LISTINGS,
    userHomeDir: "/Users/ada",
    selectedWorktreeId: null,
    onSelectWorktree: fn(),
    onCreateWorktree: fn(),
    onRemoveWorktree: fn(),
  };
}

const meta: Meta<typeof OsWorkspacesOverview> = {
  title: "systems/os/components/OsWorkspacesOverview",
  component: OsWorkspacesOverview,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "macOS Command-Tab workspace switcher: a centered glass strip of compact tiles over the shell scrim, with the focused git-backed workspace's worktrees as an always-visible vertical menu under the caption.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Three project tiles; current = check on the well; focus plate on compozy. */
export const Default: Story = {
  args: {},
  render: () => <OsWorkspacesOverview {...baseProps()} />,
};

/** Arrowing right moves the plate and caption; the check stays on compozy. */
export const FocusNonCurrent: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await body.findByTestId(`os-workspace-tile-${COMPOZY.id}`);
    await userEvent.keyboard("{ArrowRight}");
    // Home-rooted caption paths display `~`-contracted.
    await body.findByText("~/dev/branas/site");
  },
  render: () => <OsWorkspacesOverview {...baseProps()} />,
};

/** Zero project workspaces: the one primary on this surface; Global caption. */
export const Empty: Story = {
  args: {},
  render: () => (
    <OsWorkspacesOverview
      {...baseProps()}
      workspaces={[]}
      worktreesByWorkspace={{}}
      activeWorkspaceId={null}
      scope="global"
    />
  ),
};

/** The glass hugs a single tile; the add tile still trails. */
export const OneWorkspace: Story = {
  args: {},
  render: () => (
    <OsWorkspacesOverview
      {...baseProps()}
      workspaces={[COMPOZY]}
      worktreesByWorkspace={{ [COMPOZY.id]: emptyWorktreeListingFixture }}
    />
  ),
};

/** Twelve tiles stay one row; the strip fills and the track scrolls with edge fades. */
export const ManyOverflow: Story = {
  args: {},
  render: () => (
    <OsWorkspacesOverview
      {...baseProps()}
      workspaces={MANY_WORKSPACES}
      worktreesByWorkspace={{
        [COMPOZY.id]: worktreeListingFixture,
        [BRANAS.id]: emptyWorktreeListingFixture,
        [NOTES.id]: nonGitWorktreeListingFixture,
      }}
    />
  ),
};

/** Global scope on: no tile is current and the notice line renders. */
export const GlobalOn: Story = {
  args: {},
  render: () => <OsWorkspacesOverview {...baseProps()} scope="global" selectedWorktreeId={null} />,
};

/** The dashed add tile focused: caption names the directory picker. */
export const AddTileFocused: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await body.findByTestId("os-workspace-tile-add");
    await userEvent.keyboard("{End}");
    await body.findByText("Register a project folder");
  },
  render: () => <OsWorkspacesOverview {...baseProps()} />,
};

/** Canonical menu: worktree scope → check on the row, branch badge on the tile. */
export const WorktreeMenuCanonical: Story = {
  args: {},
  render: () => (
    <OsWorkspacesOverview
      {...baseProps()}
      selectedWorktreeId={worktreeReadyDirtyRunningFixture.id}
    />
  ),
};

/** Row actions popover open: Copy path + the gated Delete worktree…. */
export const WorktreeRowActionsOpen: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    const trigger = await body.findByTestId(
      `os-workspaces-worktree-actions-${worktreeReadyDirtyRunningFixture.id}`
    );
    await userEvent.click(trigger);
    await body.findByText("Copy path");
  },
  render: () => (
    <OsWorkspacesOverview
      {...baseProps()}
      selectedWorktreeId={worktreeReadyDirtyRunningFixture.id}
    />
  ),
};

/** Locked sort with inert pending/missing rows and their functional reasons. */
export const WorktreeMenuMixedStates: Story = {
  args: {},
  render: () => <OsWorkspacesOverview {...baseProps()} worktreesByWorkspace={MIXED_LISTINGS} />,
};

/** Eighteen rows all stay listed; the nest scrolls under its cap, create pinned. */
export const WorktreeMenuScrolls: Story = {
  args: {},
  render: () => (
    <OsWorkspacesOverview
      {...baseProps()}
      worktreesByWorkspace={{
        [COMPOZY.id]: scrollableWorktreesListingFixture,
        [BRANAS.id]: emptyWorktreeListingFixture,
        [NOTES.id]: nonGitWorktreeListingFixture,
      }}
    />
  ),
};

/** Git-backed with zero worktrees: only the lone dashed New worktree button. */
export const WorktreeMenuAbsence: Story = {
  args: {},
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await body.findByTestId(`os-workspace-tile-${BRANAS.id}`);
    // branas-site is git+0 → lone button; one more arrow lands on notes
    // (non-git) where the slot renders nothing at all.
    await userEvent.keyboard("{ArrowRight}");
    await body.findByTestId("os-workspaces-worktree-new");
  },
  render: () => <OsWorkspacesOverview {...baseProps()} />,
};
