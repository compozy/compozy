import type { Meta, StoryObj } from "@storybook/react-vite";
import { TriangleAlertIcon } from "lucide-react";

import {
  Alert,
  AlertAction,
  AlertActions,
  AlertDescription,
  AlertMeta,
  AlertTitle,
} from "../alert";
import { Button } from "../button";
import { Eyebrow } from "../custom/eyebrow";
import { MonoId } from "../custom/mono-id";

const meta: Meta<typeof Alert> = {
  title: "components/ui/Alert",
  component: Alert,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Static notification surface with title, description, optional icon, and action.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4 text-foreground">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
  render: () => (
    <Alert>
      <AlertTitle>Session paused</AlertTitle>
      <AlertDescription>
        The orchestrator halted execution; resume to continue from the last checkpoint.
      </AlertDescription>
    </Alert>
  ),
};

export const Danger: Story = {
  args: {},
  render: () => (
    <Alert variant="danger">
      <TriangleAlertIcon />
      <AlertTitle>Connection lost</AlertTitle>
      <AlertDescription>
        The daemon did not respond within 30 seconds. Check that it is running locally.
      </AlertDescription>
      <AlertAction>
        <Button size="sm" variant="outline">
          Retry
        </Button>
      </AlertAction>
    </Alert>
  ),
};

export const Warning: Story = {
  args: {},
  render: () => (
    <Alert variant="warning" role="status">
      <TriangleAlertIcon />
      <AlertTitle>Restart required</AlertTitle>
      <AlertDescription>Changes saved. Restart the daemon to apply them.</AlertDescription>
    </Alert>
  ),
};

export const Success: Story = {
  args: {},
  render: () => (
    <Alert variant="success" role="status">
      <AlertTitle>Deploy complete</AlertTitle>
      <AlertDescription>All services are reporting healthy.</AlertDescription>
    </Alert>
  ),
};

export const Info: Story = {
  args: {},
  render: () => (
    <Alert variant="info" role="status">
      <AlertTitle>Maintenance scheduled</AlertTitle>
      <AlertDescription>Daemon will restart automatically in 15 minutes.</AlertDescription>
    </Alert>
  ),
};

export const Accent: Story = {
  args: {},
  render: () => (
    <Alert variant="accent" role="status">
      <AlertTitle>New agent available</AlertTitle>
      <AlertDescription>claude-opus-4-7 is ready to install.</AlertDescription>
    </Alert>
  ),
};

export const WithMetaAndActions: Story = {
  args: {},
  render: () => (
    <Alert variant="warning" role="status">
      <TriangleAlertIcon />
      <AlertTitle>Provider missing</AlertTitle>
      <AlertDescription>
        This session was started with a provider that is no longer visible.
      </AlertDescription>
      <AlertMeta>
        <span className="inline-flex items-center gap-x-1.5">
          <Eyebrow>session</Eyebrow>
          <MonoId value="sess_123" />
        </span>
        <span className="inline-flex items-center gap-x-1.5">
          <Eyebrow>agent</Eyebrow>
          <MonoId value="claude-code" />
        </span>
      </AlertMeta>
      <AlertActions>
        <Button size="sm" variant="ghost">
          Dismiss
        </Button>
        <Button size="sm" variant="neutral">
          Retry
        </Button>
      </AlertActions>
    </Alert>
  ),
};
