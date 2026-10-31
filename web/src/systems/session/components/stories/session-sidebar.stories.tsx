import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { BULK_SESSION_FAMILY, BULK_SELECTED_SESSIONS } from "./session-bulk-fixtures";
import {
  GOAL_OWNER,
  HISTORICAL_JUDGES,
  LONG_AGENT,
  MIXED_RUNTIMES,
  PROFILE_OWNERS,
  PROVENANCE_FAMILY,
  STABLE_FAMILY,
  STORY_FOLDERS,
  SUBAGENT_PARENTS,
  VIEWED_SUBAGENT,
  inProfile,
  minutesAgo,
  onRuntime,
  sidebarSession,
  withSubagents,
} from "./session-sidebar-fixtures";
import { MIXED_RUNTIME_CATALOG } from "./session-sidebar-runtime-fixtures";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionSidebar } from "../session-sidebar";
import { projectGroupTreeGroups } from "../../lib/workspace-folders";
import { PROJECT_GROUPS, PROJECT_LAYOUT, PROJECT_TREE } from "./project-folder-fixtures";
import {
  sessionListCatalogFixture,
  sessionListViewFixture,
} from "../../mocks/session-list-view-fixtures";

const meta: Meta<typeof SessionSidebar> = {
  title: "systems/session/components/SessionSidebar",
  component: SessionSidebar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "In-window sessions rail with provenance threads: child sessions hang off their origin by a tree elbow. Line 1 belongs to the title; owner, provider, agent, attention words, and the branch toggle share line 2.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="flex h-[560px] w-full max-w-3xl overflow-hidden rounded-lg border border-line-strong bg-canvas">
          <div className="w-66 shrink-0 border-r border-line">
            <Story />
          </div>
          <div className="flex-1" />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Open rail with two provenance threads; the active session carries the accent
 * marker and child sessions keep their own agent identity in the sub line.
 */
export const ProvenanceThreads: Story = {
  args: {
    view: sessionListViewFixture({
      sort: "last_activity",
      setSort: fn(),
      setShelf: fn(),
      ownerOf: () => ({ id: "default", name: "default", archived: false }),
    }),
    open: true,
    sessions: PROVENANCE_FAMILY,
    disconnected: false,
    collapsedThreadIds: [],
    subagentThreadIds: [],
    currentSessionId: "sess-migration",
    onThreadBranchChange: fn(),
    onSelectSession: fn(),
    onNewSession: fn(),
    sessionActions: {
      pendingAction: null,
      pendingSessionId: null,
      onArchive: fn(),
      onDelete: fn(),
      onRename: fn(),
      onStop: fn(),
      onUnarchive: fn(),
    },
  },
};

/**
 * The first catalog page is still loading: placeholder rows in the live row's frame, a root with
 * two children on its tree elbows among them, under the real toolbar and filter. Never the
 * empty state.
 */
export const Loading: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: [],
    view: sessionListViewFixture({
      ...ProvenanceThreads.args!.view!,
      catalog: sessionListCatalogFixture({ loading: true }),
    }),
  },
};

/** Grouped by project: group headers with their first rows hold the place while the facets load. */
export const GroupsLoading: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: [],
    view: sessionListViewFixture({
      ...ProvenanceThreads.args!.view!,
      storedGroupBy: "project",
      groupBy: "project",
      groupByOptions: ["none", "project"],
      groupsLoading: true,
    }),
  },
};

/**
 * Folders from the workspace layout group root rows in Manual order; each row
 * keeps its two-line anatomy, and filed threads still hang from their elbow.
 */
export const Folders: Story = {
  args: {
    ...ProvenanceThreads.args,
    view: {
      ...ProvenanceThreads.args!.view!,
      sort: "manual",
      layout: {
        folders: STORY_FOLDERS,
        createFolder: fn(async () => STORY_FOLDERS[0]!),
        renameFolder: fn(async () => undefined),
        deleteFolder: fn(async () => undefined),
        moveFolder: fn(),
        reorderFolders: fn(),
        moveSession: fn(),
        moveSessions: fn(async () => undefined),
        reorderSessions: fn(),
      },
    },
    sessions: PROVENANCE_FAMILY.map(session =>
      session.id === "sess-migration" || session.id === "sess-empty-states"
        ? { ...session, folder_id: "fld-calm" }
        : session
    ),
  },
};

/**
 * A collapsed thread keeps the most urgent child state on its toggle — the
 * failed scrape surfaces as a danger dot beside the child count.
 */
export const CollapsedThreadSignal: Story = {
  args: {
    ...ProvenanceThreads.args,
    collapsedThreadIds: ["sess-pricing"],
    currentSessionId: "sess-deps",
  },
};

async function selectBoardSessions(canvasElement: HTMLElement) {
  const canvas = within(canvasElement);
  const user = userEvent.setup();
  await user.keyboard("{Control>}");
  for (const session of BULK_SELECTED_SESSIONS) {
    await user.click(canvas.getByTestId(`session-sidebar-session-${session.id}`));
  }
  await user.keyboard("{/Control}");
  await expect(canvas.getByTestId("session-sidebar-selection-count")).toHaveTextContent(
    "3 selected"
  );
}

export const SelectionMode: Story = {
  tags: ["play-fn"],
  args: {
    ...ProvenanceThreads.args,
    sessions: BULK_SESSION_FAMILY,
    currentSessionId: "orchestrator",
    sessionActions: {
      ...ProvenanceThreads.args!.sessionActions!,
      onStopMany: fn(),
      onArchiveMany: fn(),
      onUnarchiveMany: fn(),
      onDeleteMany: fn(),
    },
  },
  play: async ({ canvasElement }) => selectBoardSessions(canvasElement),
};

export const SelectionMenuOpen: Story = {
  ...SelectionMode,
  play: async ({ canvasElement }) => {
    await selectBoardSessions(canvasElement);
    await userEvent.click(within(canvasElement).getByTestId("session-sidebar-selection-more"));
  },
};

/**
 * Move to folder from the selection menu (#879): the folders, No folder because
 * a selected session is filed, and New folder… to create one around the
 * selection. A child row in the selection follows its parent.
 */
export const SelectionFolderMenu: Story = {
  tags: ["play-fn"],
  args: Folders.args,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const user = userEvent.setup();
    await user.keyboard("{Control>}");
    for (const id of ["sess-migration", "sess-pricing", "sess-scrape", "sess-deps"]) {
      await user.click(canvas.getByTestId(`session-sidebar-session-${id}`));
    }
    await user.keyboard("{/Control}");
    await user.click(canvas.getByTestId("session-sidebar-selection-more"));
    const body = within(canvasElement.ownerDocument.body);
    await user.click(await body.findByTestId("session-sidebar-selection-move"));
    await expect(await body.findByRole("menuitem", { name: "No folder" })).toBeVisible();
  },
};

/**
 * Subagents (nav VC-02): subagent sessions take no rows; each parent carries one
 * subagent indicator — attention, delegated while its turn is idle, the failed
 * total, or a quiet count once all settled — and plain spawned children keep
 * their thread.
 */
export const SubagentParents: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: SUBAGENT_PARENTS,
    currentSessionId: "sess-ship",
  },
};

/** Viewing a subagent reveals only that session, nested under its parent row. */
export const RevealedSubagent: Story = {
  args: {
    ...SubagentParents.args,
    currentSessionId: VIEWED_SUBAGENT.id,
    revealedSession: VIEWED_SUBAGENT,
  },
};

export const CreatedOrder: Story = {
  args: {
    ...ProvenanceThreads.args,
    view: { ...ProvenanceThreads.args!.view!, sort: "created" },
    sessions: STABLE_FAMILY,
    currentSessionId: "stable-parent",
  },
};

export const NameOrder: Story = {
  args: {
    ...CreatedOrder.args,
    view: { ...CreatedOrder.args!.view!, sort: "name" },
    sessions: [STABLE_FAMILY[2]!, STABLE_FAMILY[1]!, STABLE_FAMILY[3]!, STABLE_FAMILY[0]!],
  },
};

export const StableSortMenu: Story = {
  tags: ["play-fn"],
  args: CreatedOrder.args,
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Sort by: Created" }));
    await expect(
      await within(canvasElement.ownerDocument.body).findByRole("menuitemradio", {
        name: "Created",
      })
    ).toHaveAttribute("aria-checked", "true");
  },
};

/** Historical unfiltered catalog fixture for before/after evidence. */
export const BeforeGateFiltering: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: [GOAL_OWNER, ...HISTORICAL_JUDGES],
    currentSessionId: GOAL_OWNER.id,
  },
};

/** The default server catalog omits system rows; the Goal retains audit access. */
export const GateSessionsHidden: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: [GOAL_OWNER],
    currentSessionId: GOAL_OWNER.id,
  },
};

/**
 * Each row names its runtime with the provider's mark before the agent, in its
 * brand ink as in the runtime selector; the unbound session shows none, and a
 * child repeating its root's agent and provider collapses to one line.
 */
export const MixedRuntimes: Story = {
  parameters: { msw: { handlers: MIXED_RUNTIME_CATALOG } },
  args: {
    ...ProvenanceThreads.args,
    sessions: MIXED_RUNTIMES,
    currentSessionId: "sess-orch",
  },
};

/** Selection keeps every provider mark; the checkbox covers only the state mark. */
export const MixedRuntimesSelection: Story = {
  ...MixedRuntimes,
  tags: ["play-fn"],
  args: {
    ...MixedRuntimes.args,
    sessionActions: SelectionMode.args!.sessionActions!,
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const user = userEvent.setup();
    await user.keyboard("{Control>}");
    await user.click(canvas.getByTestId("session-sidebar-session-sess-sort"));
    await user.click(canvas.getByTestId("session-sidebar-session-sess-claim"));
    await user.keyboard("{/Control}");
    await expect(canvas.getByTestId("session-sidebar-selection-count")).toHaveTextContent(
      "2 selected"
    );
  },
};

/**
 * All profiles: the owner glyph leads the meta line of roots only, a child in
 * its root's profile repeats nothing, and the title keeps line 1 even with the
 * subagent chip and the thread fold on the row.
 */
export const AllProfiles: Story = {
  parameters: { msw: { handlers: MIXED_RUNTIME_CATALOG } },
  args: {
    ...MixedRuntimes.args,
    view: {
      ...ProvenanceThreads.args!.view!,
      aggregate: true,
      scopeLabel: "All profiles",
      ownerOf: session =>
        PROFILE_OWNERS[session.profile_id as keyof typeof PROFILE_OWNERS] ??
        PROFILE_OWNERS["00000000000000000000000000"],
    },
    sessions: [
      inProfile(
        onRuntime(
          minutesAgo(
            withSubagents(
              sidebarSession("sess-qa", "Release v0.3.1 QA cycle", "general", "running"),
              1,
              14
            ),
            0
          ),
          "claude",
          "claude-opus-5-5"
        ),
        "00000000000000000000000000"
      ),
      ...MIXED_RUNTIMES.map(session =>
        inProfile(
          session,
          session.id === "sess-cart-2" ? "profile-design" : "00000000000000000000000000"
        )
      ),
    ],
  },
};

/**
 * Bulletproof meta line: a long agent name gives way first, attention words and
 * the trailing chip and fold never truncate, notes truncate before the agent.
 */
export const LongAgentNames: Story = {
  parameters: { msw: { handlers: MIXED_RUNTIME_CATALOG } },
  args: {
    ...AllProfiles.args,
    sessions: [
      inProfile(
        onRuntime(
          minutesAgo(
            withSubagents(
              sidebarSession(
                "sess-long-root",
                "Migrate the session store to the new schema",
                LONG_AGENT,
                "needs-attention"
              ),
              12,
              148
            ),
            12
          ),
          "codex",
          "gpt-6.1-sol"
        ),
        "profile-design"
      ),
      inProfile(
        onRuntime(
          minutesAgo(
            sidebarSession(
              "sess-long-child",
              "Slice 1 — migrate session rows",
              LONG_AGENT,
              "running",
              "sess-long-root"
            ),
            1
          ),
          "codex",
          "gpt-6.1-sol"
        ),
        "profile-design"
      ),
      inProfile(
        onRuntime(
          minutesAgo(
            sidebarSession(
              "sess-long-other",
              "Slice 2 — rebuild indexes",
              "general",
              "failed",
              "sess-long-root"
            ),
            4
          ),
          "claude",
          "claude-opus-5-5"
        ),
        "profile-design"
      ),
      inProfile(
        onRuntime(
          minutesAgo(
            {
              ...sidebarSession(
                "sess-long-archived",
                "fix/silent-idle-session-reclaim-and-runtime-rebind-after-session-load",
                LONG_AGENT,
                "stopped"
              ),
              archived_at: "2026-08-06T18:00:00Z",
            },
            60 * 24 * 40
          ),
          "claude",
          "claude-sonnet-5-5"
        ),
        "00000000000000000000000000"
      ),
    ],
    currentSessionId: "sess-long-child",
  },
};

/**
 * All projects grouped by project with one profile's project folders (#1044):
 * folders sit on top and hang their projects on the same tree elbow as child
 * sessions; loose projects follow. Projects and folders drag to reorder, and
 * the quiet footer totals the whole scope by state.
 */
export const ProjectFolders: Story = {
  tags: ["play-fn"],
  args: {
    ...ProvenanceThreads.args,
    sessions: [],
    view: sessionListViewFixture({
      ...ProvenanceThreads.args!.view!,
      storedGroupBy: "project",
      groupBy: "project",
      groupByOptions: ["none", "project"],
      newFolderUnavailable: null,
      groups: projectGroupTreeGroups(PROJECT_TREE),
      collapsedGroupKeys: new Set(PROJECT_GROUPS.map(group => group.key)),
      projectFolders: { layout: PROJECT_LAYOUT, tree: PROJECT_TREE },
      totals: { needsYou: 1, working: 3, idle: 8 },
    }),
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const product = canvas.getByTestId("session-sidebar-project-folder-wfold-product");
    await expect(within(product).getByTestId("session-sidebar-group-project:ws-api")).toBeVisible();
    await expect(canvas.getByTestId("session-sidebar-totals")).toHaveTextContent(
      "1 needs you3 working8 idle"
    );
  },
};
