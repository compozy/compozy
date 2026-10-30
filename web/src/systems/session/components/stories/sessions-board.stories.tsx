import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import { expect, fn, userEvent, within } from "storybook/test";

import { storybookMswParameters } from "@/storybook/msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { workspaceBadgeIdentity } from "@/systems/workspace";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import type { SessionsBoardLane } from "../../hooks/use-sessions-board";
import type { SessionBoardColumnId } from "../../lib/session-board-contract";
import { SessionsBoard } from "../session-board/sessions-board";
import { SessionsBoardSkeleton } from "../session-board/sessions-board-skeleton";
import { SESSIONS_BOARD_SESSIONS } from "../../mocks/sessions-board-fixtures";
import { sessionsBoardView } from "./sessions-board-fixtures";

const meta: Meta<typeof SessionsBoard> = {
  title: "systems/session/components/SessionsBoard",
  component: SessionsBoard,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The session window's Kanban view: four daemon-derived columns (Needs you, Working, In review, Done) with facet counts, three-row cards, a collapsible Done column and the Archive bar.",
      },
    },
  },
  args: { onOpen: fn(), currentSessionId: "s4" },
  decorators: [
    Story => (
      <div className="flex h-170 w-full bg-canvas text-fg">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Populated: Story = {
  args: { board: sessionsBoardView() },
};

export const Loading: Story = {
  args: {
    board: sessionsBoardView(
      {
        needs_you: { loading: true, sessions: [], count: undefined },
        working: { loading: true, sessions: [], count: undefined },
        in_review: { loading: true, sessions: [], count: undefined },
        done: { loading: true, sessions: [], count: undefined },
      },
      { loading: true, count: undefined, sessions: [] }
    ),
  },
};

/** Group by Project while the lanes' facets are read: lane headers over rows of columns, as blocks. */
export const LoadingGroupedLanes: Story = {
  args: {
    board: {
      ...sessionsBoardView({}, { loading: true, count: undefined, sessions: [] }),
      columns: [],
      lanes: [],
      lanesLoading: true,
      groupBy: "project",
      groupByOptions: ["none", "project"],
    },
  },
};

/** The whole board before any of it is known: what the Sessions window shows while its code loads. */
export const BoardSkeleton: StoryObj<typeof SessionsBoardSkeleton> = {
  args: {},
  render: () => <SessionsBoardSkeleton />,
};

export const BoardSkeletonLanes: StoryObj<typeof SessionsBoardSkeleton> = {
  args: {},
  render: () => <SessionsBoardSkeleton layout="lanes" />,
};

export const Empty: Story = {
  args: {
    board: sessionsBoardView(
      {
        needs_you: { sessions: [], count: 0, hasMore: false },
        working: { sessions: [], count: 0, hasMore: false },
        in_review: { sessions: [], count: 0, hasMore: false },
        done: { sessions: [], count: 0, hasMore: false },
      },
      { count: 0, sessions: [] }
    ),
  },
};

export const ColumnError: Story = {
  args: {
    board: sessionsBoardView({ in_review: { failure: "initial", sessions: [] } }),
  },
};

export const ColumnReadErrorKeepsCards: Story = {
  args: {
    board: sessionsBoardView({
      working: { failure: "refresh" },
      done: { failure: "more" },
    }),
  },
};

export const ArchiveReadError: Story = {
  args: {
    board: sessionsBoardView({}, { count: undefined, sessions: [], failure: "initial" }),
  },
};

export const ArchiveOpenDoneCollapsed: Story = {
  tags: ["play-fn"],
  args: { board: sessionsBoardView() },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Collapse Done" }));
    await userEvent.click(canvas.getByTestId("sessions-board-archive-toggle"));
    await expect(canvas.getByTestId("sessions-board-archive-body")).toBeVisible();
  },
};

const CARD_ACTIONS: SessionLifecycleActionHandlers = {
  pendingAction: null,
  pendingSessionId: null,
  onStop: fn(),
  onRename: fn(),
  onArchive: fn(),
  onUnarchive: fn(),
  onDelete: fn(),
};

/** Each card carries the session list row's verbs behind a `…` revealed on hover or focus. */
export const CardDotsMenu: Story = {
  tags: ["play-fn"],
  args: { board: sessionsBoardView(), actions: CARD_ACTIONS },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const [trigger] = canvas.getAllByRole("button", { name: /^More actions for/ });
    await userEvent.click(trigger!);
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByRole("menuitem", { name: "Rename session" })).toBeVisible();
  },
};

/** Right-clicking a card opens the same verbs as a context menu. */
export const CardContextMenu: Story = {
  tags: ["play-fn"],
  args: { board: sessionsBoardView(), actions: CARD_ACTIONS },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const [card] = canvas.getAllByTestId(/^session-board-card-s/);
    await userEvent.pointer({ keys: "[MouseRight]", target: card! });
    const body = within(canvasElement.ownerDocument.body);
    await expect(await body.findByRole("menuitem", { name: "Delete session" })).toBeVisible();
  },
};

const LANE_NAMES = new Map([
  ["ws-core", "compozy"],
  ["ws-site", "compozy-site"],
]);
const LANE_WORKSPACES = [...LANE_NAMES.keys()];

/** Stories split the fixture cards between the two projects, alternating per column. */
function laneSessions(workspaceId: string, column: SessionBoardColumnId) {
  const index = LANE_WORKSPACES.indexOf(workspaceId);
  return SESSIONS_BOARD_SESSIONS[column]
    .filter((_, position) => position % LANE_WORKSPACES.length === index)
    .map(session => ({ ...session, workspace_id: workspaceId }));
}

function projectLane(workspaceId: string): SessionsBoardLane {
  const columns = Object.keys(SESSIONS_BOARD_SESSIONS) as SessionBoardColumnId[];
  const total = columns.reduce((sum, id) => sum + laneSessions(workspaceId, id).length, 0);
  return {
    group: {
      key: `project:${workspaceId}`,
      kind: "project",
      badge: workspaceBadgeIdentity(workspaceId, LANE_NAMES),
      total,
      working: laneSessions(workspaceId, "working").length,
      needsYou: laneSessions(workspaceId, "needs_you").length,
      scope: { kind: "project", workspaceId },
      lens: { profile: "default" },
      pair: null,
    },
    filters: { workspace_id: workspaceId, sort: "last_activity" },
  };
}

const laneHandlers = [
  compozyApiMock.get("/api/sessions/facets", ({ request }) => {
    const workspaceId = new URL(request.url).searchParams.get("workspace_id") ?? "";
    const count = (id: SessionBoardColumnId) => laneSessions(workspaceId, id).length;
    const board = {
      needs_you: count("needs_you"),
      working: count("working"),
      in_review: count("in_review"),
      done: count("done"),
    };
    const all = board.needs_you + board.working + board.in_review + board.done;
    return HttpResponse.json({
      facets: {
        terminal_approvals: 0,
        all,
        needs_you: board.needs_you,
        working: board.working,
        finished: board.done,
        idle: 0,
        board,
      },
      by_workspace: [],
      by_profile: [],
    });
  }),
  compozyApiMock.get("/api/sessions", ({ request }) => {
    const url = new URL(request.url);
    const column = url.searchParams.get("board_column") as SessionBoardColumnId | null;
    const sessions = column ? laneSessions(url.searchParams.get("workspace_id") ?? "", column) : [];
    return HttpResponse.json({ sessions, page: { has_more: false, limit: 50 } });
  }),
];

/**
 * Group by Project: one swimlane per project, each holding the same four
 * columns read through that project's own scope (GitHub Projects style).
 */
export const GroupedByProject: Story = {
  args: {
    board: {
      ...sessionsBoardView(),
      columns: [],
      lanes: LANE_WORKSPACES.map(projectLane),
      groupBy: "project",
      groupByOptions: ["none", "project"],
    },
    actions: CARD_ACTIONS,
  },
  parameters: storybookMswParameters({ session: laneHandlers }),
};
