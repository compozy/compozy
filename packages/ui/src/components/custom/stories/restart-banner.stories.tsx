import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn } from "storybook/test";

import { RestartBanner } from "../restart-banner";

const meta: Meta<typeof RestartBanner> = {
  title: "components/custom/RestartBanner",
  component: RestartBanner,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          'Warm-orange "Restart required to apply" banner Pure-presentation slot: the consumer maps its state machine to `tone` / `message` / `detail` / `busy` props and decides whether to render. Optional `restartNow` and `onDismiss` callbacks wire the inline buttons.',
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Idle — banner renders without the action button (no `restartNow` wired). */
export const Idle: Story = {
  args: {},
};

/** RestartNowActive — banner shows the warm-orange "Restart now" button. */
export const RestartNowActive: Story = {
  args: {
    restartNow: fn(),
  },
};

/** Pending — `isPending` disables the action button and swaps the label to "Starting...". */
export const Pending: Story = {
  args: {
    restartNow: fn(),
    isPending: true,
  },
};

/** Polling — info tone with a spinner glyph and an inline operation-id chip. */
export const Polling: Story = {
  args: {
    tone: "info",
    busy: true,
    message: "Restarting · stopping",
    detail: <span className="font-mono text-form-hint text-muted">op_abcdef</span>,
  },
};

/** Failure — danger tone with a dismiss button. */
export const Failure: Story = {
  args: {
    tone: "danger",
    message: "Restart failed: helper exited non-zero",
    onDismiss: fn(),
  },
};

/** CustomMessage — replaces the default copy via the `message` prop. */
export const CustomMessage: Story = {
  args: {
    restartNow: fn(),
    message: "Provider config changed. Restart to apply across active sessions.",
  },
};

/** WithDescription — title plus a second line, a dismiss, and the primary restart action. */
export const WithDescription: Story = {
  args: {
    message: "Restart needed",
    description:
      "Some saved changes apply after a restart. Running sessions keep their current rules until then.",
    restartNow: fn(),
    onDismiss: fn(),
    dismissLabel: "Not now",
  },
};
