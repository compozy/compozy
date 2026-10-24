import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { SessionsBoard } from "../session-board/sessions-board";
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
      { count: 0, sessions: [] }
    ),
  },
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
