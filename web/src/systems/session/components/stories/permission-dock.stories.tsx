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

/**
 * VC-10 — a terminal ask, decided on the session's one decision surface.
 *
 * The generic subject line cannot show what matters here, so the terminal
 * detail replaces it: the exact command as the agent wrote it, the folder it
 * would run in, and the terminal it is bound to.
 *
 * The board titles this row "Claude Code wants to run · dev server". The title
 * takes the agent's name from the session when the cache has it — here it does
 * not, so the sentence falls back to "The agent" — and the request carries only
 * a `terminal_id`, so the terminal's display name stays out of the title.
 * Authorized runtime-truth delta.
 */
export const TerminalExec: Story = {
  args: {
    permission: {
      ...permission,
      toolName: "compozy__terminal_exec",
      toolInput: {
        command: "bun",
        args: ["add", "@xterm/xterm", "@xterm/addon-fit"],
        cwd: "~/dev/atlas-api",
        terminal_id: "term-4f21c9a03b7e",
        // The runtime's own classification; without it the client fails closed
        // to `unclassifiable` and withholds the remembered decision.
        risk: "ordinary",
      },
    },
  },
};

/**
 * VC-11 — an irreversible command.
 *
 * Danger is stated in words before the command is even read, and no remembered
 * decision is offered: the fixed irreversible set always asks, at every autonomy
 * level, so an "Always allow" here would be a promise the runtime would refuse
 * to keep.
 */
export const TerminalIrreversible: Story = {
  args: {
    permission: {
      ...permission,
      toolName: "compozy__terminal_exec",
      toolInput: {
        command: "rm",
        args: ["-rf", "/var/lib/atlas/journal-backups"],
        cwd: "~/dev/atlas-api",
        terminal_id: "term-4f21c9a03b7e",
        risk: "irreversible",
      },
    },
  },
};

/**
 * VC-12a — permission to type into a terminal someone else is watching.
 *
 * Typing is scoped to one terminal, named by id, and reads as a permission
 * rather than as a tool call.
 */
export const TerminalTypingGrant: Story = {
  args: {
    permission: {
      ...permission,
      toolName: "compozy__terminal_write",
      toolInput: { terminal_id: "term-9cd7e14b2a66" },
    },
  },
};
