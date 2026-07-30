import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import type { NetworkParticipationDraft } from "@/lib/network-participation";
import { agentFixtures } from "@/systems/agent/mocks";
import { workspaceDetailFixture } from "@/systems/workspace/mocks";

import { SessionCreateDialog } from "../session-create-dialog";

const workspace = workspaceDetailFixture.workspace;

const localParticipation = {
  mode: "local",
  channelStrategy: "",
  channelId: "",
} satisfies NetworkParticipationDraft;

const liveParticipation = {
  mode: "live",
  channelStrategy: "named",
  channelId: "release-room",
} satisfies NetworkParticipationDraft;

const baseArgs = {
  open: true,
  onOpenChange: fn(),
  mode: "simple" as const,
  onModeChange: fn(),
  agents: agentFixtures,
  workspace,
  workspaces: [{ id: workspace.id, name: workspace.name, root_dir: workspace.root_dir }],
  workspaceId: workspace.id,
  userHomeDir: undefined,
  onWorkspaceChange: fn(),
  sessionName: "Investigate checkout latency",
  onSessionNameChange: fn(),
  workspacePath: "",
  onWorkspacePathChange: fn(),
  selectedAgentName: agentFixtures[0]?.name ?? "",
  networkParticipation: localParticipation,
  onAgentChange: fn(),
  onNetworkParticipationChange: fn(),
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
          "Session creation starts durable work without fabricating a first message. Simple selects the agent; Advanced reveals workspace and launch details. Runtime choices belong to the session composer.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01: the common path exposes the agent and one clear durable-start action. */
export const Simple: Story = { args: baseArgs };

/** VC-02: the sole disclosure tier contains workspace, name, path, and Network participation. */
export const Advanced: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    workspacePath: "services/checkout",
  },
};

/** VC-03: explicit Live participation is valid only with its named channel. */
export const LiveParticipation: Story = {
  args: {
    ...baseArgs,
    mode: "advanced" as const,
    networkParticipation: liveParticipation,
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
    await expect(await body.findByTestId("session-create-workspace-select")).toBeVisible();
  },
};
