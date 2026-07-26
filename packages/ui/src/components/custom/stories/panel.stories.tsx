import type { Meta, StoryObj } from "@storybook/react-vite";

import { Button } from "../../button";
import { Panel } from "../panel";
import { Pill } from "../pill";

const meta: Meta<typeof Panel> = {
  title: "components/custom/Panel",
  component: Panel,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Flat panel container behind dashboard zones and grouped rows. Optional hairline head (title/meta/right) and pinned foot; body padding overridable for full-bleed row lists.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    title: "Queue health",
    meta: "last 14 days",
    children: <p className="text-small-body text-muted">Panel body content.</p>,
  },
};

export const WithRightAction: Story = {
  render: () => (
    <Panel
      title="Active runs"
      meta="4"
      right={
        <Button size="sm" variant="ghost">
          View all
        </Button>
      }
    >
      <p className="text-small-body text-muted">Run rows render here.</p>
    </Panel>
  ),
};

export const HeadlessRows: Story = {
  render: () => (
    <Panel bodyClassName="p-0">
      {["Deploy docs site", "Competitor research", "Overnight writer session"].map(row => (
        <div
          key={row}
          className="flex items-center gap-2 border-t border-line-soft px-4 py-3 text-small-body text-muted first:border-t-0"
        >
          <Pill.Dot tone="warning" />
          {row}
        </div>
      ))}
    </Panel>
  ),
};

export const WithFoot: Story = {
  render: () => (
    <div className="flex h-56">
      <Panel
        className="flex-1"
        bodyClassName="p-0"
        foot={
          <Button size="sm" variant="ghost">
            View network
          </Button>
        }
      >
        <div className="px-4 py-3 text-small-body text-muted">Peers online · 2</div>
        <div className="border-t border-line-soft px-4 py-3 text-small-body text-muted">
          Open work · 3
        </div>
      </Panel>
    </div>
  ),
};
