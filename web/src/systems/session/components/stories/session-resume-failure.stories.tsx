import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import { SessionResumeFailure } from "../session-resume-failure";

const meta: Meta<typeof SessionResumeFailure> = {
  title: "systems/session/components/SessionResumeFailure",
  component: SessionResumeFailure,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Inline alert shown when a session attach attempt fails.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-3xl">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Missing provider copy gives operators a concrete remediation path.
 */
export const MissingProvider: Story = {
  args: {
    sessionId: "session_launch_coordination",
    message: "Provider is unavailable.",
    missingProvider: "codex",
    agentName: "frontend-launch",
    isRetrying: false,
    onRetry: fn(),
    onDismiss: fn(),
  },
};

/**
 * Generic failure renders the daemon-provided message.
 */
export const GenericFailure: Story = {
  args: {
    sessionId: "session_launch_coordination",
    message: "Session process exited before the ACP handshake completed.",
    missingProvider: null,
    agentName: "frontend-launch",
    isRetrying: false,
    onRetry: fn(),
    onDismiss: fn(),
  },
};

/**
 * Retrying disables the retry button and swaps in a spinner.
 */
export const Retrying: Story = {
  args: {
    sessionId: "session_launch_coordination",
    message: "Provider is unavailable.",
    missingProvider: "codex",
    agentName: "frontend-launch",
    isRetrying: true,
    onRetry: fn(),
    onDismiss: fn(),
  },
};
