import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { SessionSidebar } from "../session-sidebar";
import sidebarMeta, { ProvenanceThreads } from "./session-sidebar.stories";
import { CONTINUED_FAMILY, ORIGIN_SESSIONS } from "./session-sidebar-fixtures";

const meta: Meta<typeof SessionSidebar> = {
  ...sidebarMeta,
  title: "systems/session/components/SessionSidebar/Origins",
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Rows titled by who started them: an automation run and an always-on main show their agent,
 * an operator's untitled session keeps "New session", and the compact times keep counting.
 */
export const SessionOrigins: Story = {
  tags: ["play-fn"],
  args: {
    ...ProvenanceThreads.args,
    sessions: ORIGIN_SESSIONS,
    currentSessionId: "sess-operator-titled",
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByTestId("session-sidebar-session-sess-triage-run")).toHaveTextContent(
      "issue-triage"
    );
    await expect(canvas.getByTestId("session-sidebar-session-sess-log-triage")).toHaveTextContent(
      "log-triage"
    );
    await expect(
      canvas.getByTestId("session-sidebar-session-sess-operator-untitled")
    ).toHaveTextContent("New session");
  },
};

/**
 * Issue #1644: workers a daemon restart stopped and their orchestrator continued
 * stay in its thread, each continuation beside its stopped predecessor, through a
 * repeated continuation. A continued top-level session stays its own row.
 */
export const ContinuedChildren: Story = {
  args: {
    ...ProvenanceThreads.args,
    sessions: CONTINUED_FAMILY,
    currentSessionId: "sess-s3-next",
  },
};

/** The folded orchestrator thread still signals its running continuations. */
export const ContinuedChildrenCollapsed: Story = {
  args: {
    ...ContinuedChildren.args,
    collapsedThreadIds: ["sess-conductor"],
    currentSessionId: "sess-notes-next",
  },
};
