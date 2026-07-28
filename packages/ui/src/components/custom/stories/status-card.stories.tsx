import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../../button";
import { StatusCard } from "../status-card";

const meta: Meta<typeof StatusCard> = {
  title: "components/custom/StatusCard",
  component: StatusCard,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component: "Compound status surface with header, body, footer, and action slots.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Shows the complete status-card composition with an actionable warning. */
export const Default: Story = {
  args: {},
  render: () => (
    <StatusCard className="w-96" tone="warning">
      <StatusCard.Header label="Delivery degraded" />
      <StatusCard.Body>The last delivery attempt returned a retryable error.</StatusCard.Body>
      <StatusCard.Footer>
        <StatusCard.Action>
          <Button size="sm" type="button" variant="outline">
            Retry delivery
          </Button>
        </StatusCard.Action>
      </StatusCard.Footer>
    </StatusCard>
  ),
};
