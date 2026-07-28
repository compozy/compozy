import type { Meta, StoryObj } from "@storybook/react-vite";

import { DayAreaChart } from "../day-area-chart";

const meta: Meta<typeof DayAreaChart> = {
  title: "components/custom/DayAreaChart",
  component: DayAreaChart,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Single-series per-day area chart in neutral data ink with lazy recharts — magnitude never borrows signal colors.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const DAYS = Array.from({ length: 30 }, (_, index) => ({
  date: `Jun ${index + 1}`,
  value: 20_000 + ((index * 37) % 13) * 4_000,
}));

export const Default: Story = {
  args: {
    ariaLabel: "Tokens per day over the selected window",
    data: DAYS,
  },
};

export const WithFormattedTooltip: Story = {
  args: {
    ariaLabel: "Tokens per day with estimated cost",
    data: DAYS,
    formatValue: value => `${Math.round(value / 1000)}K tokens`,
    height: 96,
  },
};
