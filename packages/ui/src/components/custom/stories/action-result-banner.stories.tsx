import type { Meta, StoryObj } from "@storybook/react-vite";
import { CircleCheckIcon, OctagonXIcon, TriangleAlertIcon } from "lucide-react";

import { Button } from "@agh/ui";
import { ActionResultBanner } from "../action-result-banner";

const meta: Meta<typeof ActionResultBanner> = {
  title: "components/custom/ActionResultBanner",
  component: ActionResultBanner,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Compact result banner used after a destructive or asynchronous action completes. Tones map to the desaturated AGH signal palette (success / warning / danger / info / neutral) and render at 6–10% tint.",
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[480px] bg-background p-4">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Success after a successful save — desaturated success tint, no saturated banner.
 */
export const Success: Story = {
  args: {},
  render: () => (
    <ActionResultBanner
      tone="success"
      icon={CircleCheckIcon}
      title="Workspace saved"
      description="Settings will reload on the next session."
    />
  ),
};

/**
 * Warning with inline actions — keeps the warning tint quiet behind the title.
 */
export const Warning: Story = {
  args: {},
  render: () => (
    <ActionResultBanner
      tone="warning"
      icon={TriangleAlertIcon}
      title="Provider quota nearing limit"
      description="The agent will throttle requests once the daily budget is reached."
      actions={
        <Button size="xs" variant="outline">
          Review
        </Button>
      }
    />
  ),
};

/**
 * Danger reserved for hard failures the operator must acknowledge.
 */
export const Danger: Story = {
  args: {},
  render: () => (
    <ActionResultBanner
      tone="danger"
      icon={OctagonXIcon}
      title="Bridge handshake failed"
      description="The remote peer rejected the agh-network/v0 greet."
      actions={
        <Button size="xs" variant="outline">
          Retry
        </Button>
      }
    />
  ),
};

/**
 * Neutral — quiet status row when no signal tone applies.
 */
export const Neutral: Story = {
  args: {},
  render: () => (
    <ActionResultBanner
      tone="neutral"
      title="No changes to apply"
      description="The provider configuration matches the local snapshot."
    />
  ),
};
