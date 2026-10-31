import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, within } from "storybook/test";

import { SESSIONS_BOARD_LATER } from "../../mocks/sessions-board-fixtures";
import { sessionListViewFixture } from "../../mocks/session-list-view-fixtures";
import { SessionSidebar } from "../session-sidebar";
import sidebarMeta, { ProvenanceThreads } from "./session-sidebar.stories";

const meta: Meta<typeof SessionSidebar> = {
  ...sidebarMeta,
  title: "systems/session/components/SessionSidebar/Later",
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The active list with sessions in Later: the bookmark toggle beside Archive carries their count. */
export const LaterCount: Story = {
  args: {
    ...ProvenanceThreads.args,
    view: sessionListViewFixture({ ...ProvenanceThreads.args!.view!, laterCount: 3 }),
  },
};

/**
 * The Later view: each parked row says why it waits, when it comes back and when it was parked,
 * with a badge for agent messages waiting on it. Its menu offers Back from Later.
 */
export const LaterView: Story = {
  tags: ["play-fn"],
  args: {
    ...ProvenanceThreads.args,
    sessions: SESSIONS_BOARD_LATER,
    currentSessionId: undefined,
    view: sessionListViewFixture({
      ...ProvenanceThreads.args!.view!,
      shelf: "later",
      laterCount: SESSIONS_BOARD_LATER.length,
    }),
    sessionActions: {
      ...ProvenanceThreads.args!.sessionActions!,
      onMoveToLater: fn(),
      onBackFromLater: fn(),
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("session-sidebar-later")).toHaveAttribute(
      "aria-pressed",
      "true"
    );
    await expect(canvas.getByTestId("session-later-pending-l2")).toHaveAttribute(
      "title",
      "2 messages waiting"
    );
  },
};
