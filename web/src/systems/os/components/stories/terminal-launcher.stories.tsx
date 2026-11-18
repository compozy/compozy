import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";
import { TERMINAL_FIXTURES } from "@/systems/terminal/mocks/terminal-fixtures";

import {
  defaultTerminalLauncherDestination,
  runningTerminals,
  terminalLauncherDestinations,
} from "../../lib/terminal-launcher-model";
import { TerminalLauncher } from "../terminal-launcher";

const WORKSPACES = [
  { id: "ws-atlas", name: "atlas-api" },
  { id: "ws-beacon", name: "beacon-web" },
  { id: "ws-relay", name: "relay" },
];

const worktree = (id: string, workspaceId: string, name: string) => ({
  id,
  workspace_id: workspaceId,
  name,
  branch: `feat/${name}`,
  state: "ready",
  profile_name: "work",
});

const WORKTREES = [
  worktree("wt-login", "ws-atlas", "feature-login"),
  worktree("wt-retry", "ws-atlas", "fix-retry-backoff"),
  worktree("wt-docs", "ws-atlas", "docs-refresh"),
  worktree("wt-billing", "ws-atlas", "billing-export"),
  worktree("wt-landing", "ws-beacon", "landing-copy"),
  worktree("wt-login-web", "ws-beacon", "login-form"),
];

const DESTINATIONS = terminalLauncherDestinations(WORKSPACES, WORKTREES, "work");

const meta: Meta<typeof TerminalLauncher> = {
  title: "systems/os/components/TerminalLauncher",
  component: TerminalLauncher,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "The All projects terminal picker. Each project's worktrees fold under it on the branched tree connector the session list uses for child sessions: the fold control or → and ← unfold and fold the selected project, and a search unfolds the projects holding a matching worktree.",
      },
    },
  },
  args: {
    destinations: DESTINATIONS,
    defaultDestination: defaultTerminalLauncherDestination(DESTINATIONS, "ws-atlas"),
    running: runningTerminals(TERMINAL_FIXTURES),
    projectNames: new Map(WORKSPACES.map(workspace => [workspace.id, workspace.name])),
    onOpenNew: fn(),
    onAttach: fn(),
    className: "w-[32rem] rounded-lg border border-line",
  },
  decorators: [
    Story => (
      <CenteredSurface className="p-6">
        <Story />
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** At rest: every project folded, the remembered one selected for a one-key open. */
export const Collapsed: Story = {};

/** The fold control unfolds a project: its worktrees hang from the project's rail. */
export const Expanded: Story = {
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("terminal-launcher-fold-ws-atlas"));
    await expect(canvas.getByTestId("terminal-launcher-new-ws-atlas")).toHaveAttribute(
      "aria-expanded",
      "true"
    );
  },
};

/** A search unfolds the projects holding a match to the matching worktrees; a project that only holds a match is dimmed context, and the first matching worktree is selected. */
export const SearchExpanded: Story = {
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.type(canvas.getByPlaceholderText("Open a terminal in…"), "login");
    await expect(canvas.getByTestId("terminal-launcher-new-ws-beacon")).toHaveAttribute(
      "aria-expanded",
      "true"
    );
  },
};
