import type { Meta, StoryObj } from "@storybook/react-vite";

import { DayStackedBars } from "../day-stacked-bars";

const meta: Meta<typeof DayStackedBars> = {
  title: "components/custom/DayStackedBars",
  component: DayStackedBars,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Stacked per-day bars with lazy recharts. Fills come from the caller's typed series; use signal colors only when the series itself is that state.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const DAYS = [
  { date: "Jul 10", completed: 7, canceled: 1, failed: 0 },
  { date: "Jul 11", completed: 13, canceled: 0, failed: 1 },
  { date: "Jul 12", completed: 6, canceled: 0, failed: 0 },
  { date: "Jul 13", completed: 5, canceled: 1, failed: 0 },
  { date: "Jul 14", completed: 11, canceled: 0, failed: 1 },
  { date: "Jul 15", completed: 15, canceled: 0, failed: 0 },
  { date: "Jul 16", completed: 14, canceled: 1, failed: 1 },
];

export const Outcomes: Story = {
  args: {
    ariaLabel: "Run outcomes per day",
    data: DAYS,
    series: [
      { key: "completed", fill: "var(--color-success)", label: "completed" },
      { key: "canceled", fill: "var(--color-viz-other)", label: "canceled" },
      { key: "failed", fill: "var(--color-danger)", label: "failed" },
    ],
  },
};

export const NeutralSingleSeries: Story = {
  args: {
    ariaLabel: "Events per day",
    data: DAYS.map(day => ({ date: day.date, events: day.completed + day.failed })),
    series: [{ key: "events", fill: "var(--color-viz-bar)", label: "events" }],
    height: 96,
  },
};
