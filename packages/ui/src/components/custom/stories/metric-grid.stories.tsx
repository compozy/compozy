import type { Meta, StoryObj } from "@storybook/react-vite";

import { Metric } from "../metric";
import { MetricGrid } from "../metric-grid";

const meta: Meta<typeof MetricGrid> = {
  title: "components/custom/MetricGrid",
  component: MetricGrid,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Metric grid host for responsive rows of Metric cards. Defaults to 1/2/4 columns.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const metrics = [
  <Metric key="sessions" label="Sessions" value="12" subtext="4 active" />,
  <Metric key="providers" label="Providers" value="08" tone="success" subtext="All installed" />,
  <Metric key="queued" label="Queued" value="03" tone="warning" subtext="2 waiting" />,
  <Metric key="failures" label="Failures" value="00" tone="danger" subtext="Last 24h" />,
];

export const FourColumns: Story = {
  args: {},
  render: () => <MetricGrid>{metrics}</MetricGrid>,
};

export const ThreeColumns: Story = {
  args: {},
  render: () => <MetricGrid columns={3}>{metrics.slice(0, 3)}</MetricGrid>,
};

export const TwoColumns: Story = {
  args: {},
  render: () => <MetricGrid columns={2}>{metrics.slice(0, 2)}</MetricGrid>,
};

export const OneColumn: Story = {
  args: {},
  render: () => <MetricGrid columns={1}>{metrics.slice(0, 2)}</MetricGrid>,
};
