import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState, type ComponentProps } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { sessionListViewFixture } from "../../mocks/session-list-view-fixtures";
import { SessionSidebar } from "../session-sidebar";
import { BRANCH_SESSIONS, SUBAGENT_BRANCH_HANDLERS } from "./session-sidebar-branch-fixtures";

const meta: Meta<typeof SessionSidebar> = {
  title: "systems/session/components/SessionSidebar/Branches",
  component: SessionSidebar,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Issue #919: a session row's branch — one indicator per kind of descendant (child sessions, subagents) and one chevron; with both kinds, a segmented control whose pill travels with the sliding list.",
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

/** The rail's branch flags held the way the sidebar store holds them, so stories can switch. */
function BranchingSidebar(args: ComponentProps<typeof SessionSidebar>) {
  const [collapsed, setCollapsed] = useState(args.collapsedThreadIds);
  const [subagents, setSubagents] = useState(args.subagentThreadIds);
  return (
    <SessionSidebar
      {...args}
      collapsedThreadIds={collapsed}
      subagentThreadIds={subagents}
      onThreadBranchChange={(sessionId, segment) => {
        args.onThreadBranchChange(sessionId, segment);
        setCollapsed(ids => [
          ...ids.filter(id => id !== sessionId),
          ...(segment === "children" ? [] : [sessionId]),
        ]);
        setSubagents(ids => [
          ...ids.filter(id => id !== sessionId),
          ...(segment === "subagents" ? [sessionId] : []),
        ]);
      }}
    />
  );
}

/**
 * Issue #919 branch shapes: child sessions only (one indicator and chevron),
 * subagents only (folded: nothing is read), both (a segmented control with the
 * rubber pill), and many subagents. Click an indicator to unfold its list.
 */
export const BranchIndicators: Story = {
  parameters: { msw: { handlers: SUBAGENT_BRANCH_HANDLERS } },
  render: args => <BranchingSidebar {...args} />,
  args: {
    view: sessionListViewFixture({
      sort: "last_activity",
      setSort: fn(),
      setShelf: fn(),
      ownerOf: () => ({ id: "default", name: "default", archived: false }),
    }),
    open: true,
    disconnected: false,
    subagentThreadIds: [],
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
    sessions: BRANCH_SESSIONS,
    collapsedThreadIds: ["sess-ship"],
    currentSessionId: "sess-docs",
  },
};

/** Only subagents: the open branch mounts one page, then "Show N more". */
export const SubagentBranchOpen: Story = {
  ...BranchIndicators,
  args: {
    ...BranchIndicators.args,
    collapsedThreadIds: ["sess-ship", "sess-audit"],
    subagentThreadIds: ["sess-audit"],
  },
};

/** 24 subagents: eight rows mount, the rest wait behind "Show 8 more". */
export const ManySubagents: Story = {
  ...BranchIndicators,
  tags: ["play-fn"],
  args: {
    ...BranchIndicators.args,
    collapsedThreadIds: ["sess-ship", "sess-migrate"],
    subagentThreadIds: ["sess-migrate"],
  },
  play: async ({ canvasElement }) => {
    const branch = within(
      await within(canvasElement).findByTestId("session-sidebar-subagents-sess-migrate")
    );
    await waitFor(() => expect(branch.getAllByRole("button", { name: /^Open / })).toHaveLength(8));
    await userEvent.click(branch.getByRole("button", { name: "Show 8 more" }));
    await waitFor(() => expect(branch.getAllByRole("button", { name: /^Open / })).toHaveLength(16));
  },
};

/**
 * Both kinds: the segmented control. Choosing subagents slides the list left
 * as the pill stretches right; choosing child sessions slides back; pressing
 * the selected segment folds the branch.
 */
export const SegmentedSwitch: Story = {
  ...BranchIndicators,
  tags: ["play-fn"],
  args: { ...BranchIndicators.args, collapsedThreadIds: [] },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const subagents = canvas.getByTestId("session-sidebar-branch-subagents-sess-ship");
    await userEvent.click(subagents);
    await expect(subagents).toHaveAttribute("aria-expanded", "true");
    await expect(
      await canvas.findByTestId("session-sidebar-subagents-sess-ship")
    ).toBeInTheDocument();
  },
};
