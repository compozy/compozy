import type { Meta, StoryObj } from "@storybook/react-vite";

import { CenteredSurface } from "@/storybook/story-layout";
import type { PermissionRequest } from "@/systems/session";

import { PermissionDock } from "../permission-dock";

const permission: PermissionRequest = {
  requestId: "req-123",
  toolName: "Run a command",
  action: "execute",
  resource: "workspace compozy",
  toolInput: { command: "bunx turbo run lint typecheck test --filter=./web" },
  turnId: "turn-001",
};

const meta: Meta<typeof PermissionDock> = {
  title: "systems/session/components/PermissionDock",
  component: PermissionDock,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Pending permission as a composer-docked decision panel. Buttons render only for the decisions the runtime offers; reject-always lives behind the reject split; digit keys 1–4 decide directly (key 4 fires with the menu closed).",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
  args: {
    sessionId: "sess-001",
    workspaceId: "ws_alpha",
    countLabel: null,
    onResolved: () => {},
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const AllFourDecisions: Story = {
  args: { permission },
};

export const QueuedCount: Story = {
  args: { permission, countLabel: "1/2" },
};

export const GatedDecisions: Story = {
  parameters: {
    docs: {
      description: {
        story: "Runtimes that offer only a subset of decisions gate the buttons and keys.",
      },
    },
  },
  args: {
    permission: {
      ...permission,
      supportedDecisions: ["allow-once", "reject-once"],
    },
  },
};
