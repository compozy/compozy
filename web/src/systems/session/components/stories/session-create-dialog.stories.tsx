import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";
import { agentFixtures } from "@/systems/agent/mocks";
import { workspaceDetailFixture } from "@/systems/workspace/mocks";

import { SessionCreateDialog } from "../session-create-dialog";

const workspace = workspaceDetailFixture.workspace;

const baseArgs = {
  open: true,
  restoreFocusOnClose: true,
  onOpenChange: fn(),
  mode: "simple" as const,
  onModeChange: fn(),
  agents: agentFixtures,
  destinationLabel: workspace.name,
  sessionRoot: workspace.root_dir,
  destinationReady: true,
  sessionName: "Investigate checkout latency",
  onSessionNameChange: fn(),
  selectedAgentName: agentFixtures[0]?.name ?? "",
  onAgentChange: fn(),
  onSubmit: fn(),
  isSubmitting: false,
  submitError: null,
};

const meta: Meta<typeof SessionCreateDialog> = {
  title: "systems/session/components/SessionCreateDialog",
  component: SessionCreateDialog,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Session creation starts durable work without fabricating a first message. Simple selects the agent; Advanced reveals name and environment. Runtime choices belong to the session composer.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01: the common path exposes the agent and one clear durable-start action. */
export const Simple: Story = { args: baseArgs };

/** VC-08: under All projects and All profiles the row asks "In project ▾ · as profile ▾". */
export const AllProjectsAndProfiles: Story = {
  args: {
    ...baseArgs,
    destinationAxes: {
      project: {
        options: [
          { value: "ws_alpha", label: "alpha" },
          { value: "@user", label: "User" },
        ],
        value: "ws_alpha",
        onChange: () => undefined,
      },
      profile: {
        options: [
          { value: "default", label: "default" },
          { value: "marketing", label: "marketing" },
        ],
        value: "default",
        onChange: () => undefined,
      },
    },
  },
};

/** VC-08: under All projects with one profile only the project is asked. */
export const AllProjects: Story = {
  args: {
    ...baseArgs,
    destinationAxes: {
      project: {
        options: [
          { value: "ws_alpha", label: "alpha" },
          { value: "@user", label: "User" },
        ],
        value: "ws_alpha",
        onChange: () => undefined,
      },
    },
  },
};

/** VC-02: the sole disclosure tier contains name and environment. */
export const Advanced: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
  },
};

/** VC-04: durable acceptance blocks duplicate submission and dismissal. */
export const Pending: Story = {
  args: { ...baseArgs, isSubmitting: true },
};

/** VC-05: a creation failure remains attached to the dialog the operator can correct. */
export const SubmitError: Story = {
  args: { ...baseArgs, submitError: "Session creation was not accepted. Try again." },
};

/** VC-06: the mobile shell keeps the primary action reachable at the bottom surface. */
export const Mobile: Story = {
  args: { ...baseArgs, mode: "advanced" as const },
  parameters: { viewport: { defaultViewport: "iphone14" } },
};

/** The toolbar remains keyboard-operable and exposes the Advanced fields on demand. */
export const AdvancedKeyboard: Story = {
  args: baseArgs,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("session-create-mode-advanced"));
    await expect(await body.findByTestId("session-create-name-input")).toBeVisible();
    await expect(await body.findByTestId("workspace-scope-statement")).toBeVisible();
  },
};
