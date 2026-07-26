import type { Meta, StoryObj } from "@storybook/react-vite";

import { Tabs, TabsContent, TabsList, TabsTrigger } from "../tabs";

const meta: Meta<typeof Tabs> = {
  title: "components/ui/Tabs",
  component: Tabs,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "Page-head filter row. Triggers are separated by a `·` glyph; counts render inline as bare 10.5px mono `--faint` (no chip). Segmented-control surfaces use `<PillGroup>` instead.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const panels = [
  {
    value: "events",
    label: "Events",
    body: "Append-only ACP event stream replayed from sessiondb.",
  },
  {
    value: "metrics",
    label: "Metrics",
    body: "Latency, error rate, and token counters per driver.",
  },
  {
    value: "artifacts",
    label: "Artifacts",
    body: "Captured file snapshots grouped by turn.",
  },
] as const;

export const Default: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          "Default lane row — Triggers separated by `·`; counts render inline as bare 10.5px mono `--faint` (no chip).",
      },
    },
  },
  render: () => (
    <Tabs defaultValue="all" className="w-xl">
      <TabsList>
        <TabsTrigger count={42} value="all">
          All
        </TabsTrigger>
        <TabsTrigger count={6} value="active">
          Active
        </TabsTrigger>
        <TabsTrigger count={3} value="waiting">
          Waiting
        </TabsTrigger>
        <TabsTrigger count={11} value="done">
          Done
        </TabsTrigger>
      </TabsList>
      <TabsContent value="all" className="pt-3">
        <p className="text-sm text-muted-foreground">Showing 42 tasks across every status.</p>
      </TabsContent>
      <TabsContent value="active" className="pt-3">
        <p className="text-sm text-muted-foreground">6 tasks in flight right now.</p>
      </TabsContent>
      <TabsContent value="waiting" className="pt-3">
        <p className="text-sm text-muted-foreground">3 tasks waiting on dependencies.</p>
      </TabsContent>
      <TabsContent value="done" className="pt-3">
        <p className="text-sm text-muted-foreground">11 tasks completed in the last 24h.</p>
      </TabsContent>
    </Tabs>
  ),
};

export const WithoutCounts: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story: "Lane row without counts — separator + active underline still apply.",
      },
    },
  },
  render: () => (
    <Tabs defaultValue="overview" className="w-xl">
      <TabsList>
        <TabsTrigger value="overview">Overview</TabsTrigger>
        <TabsTrigger value="usage">Usage</TabsTrigger>
        <TabsTrigger value="audit">Audit log</TabsTrigger>
      </TabsList>
      <TabsContent value="overview" className="pt-3">
        <p className="text-sm text-muted-foreground">Workspace summary.</p>
      </TabsContent>
      <TabsContent value="usage" className="pt-3">
        <p className="text-sm text-muted-foreground">Session and token usage.</p>
      </TabsContent>
      <TabsContent value="audit" className="pt-3">
        <p className="text-sm text-muted-foreground">Recent operator and agent actions.</p>
      </TabsContent>
    </Tabs>
  ),
};

export const Vertical: Story = {
  args: {},
  parameters: {
    docs: {
      description: {
        story:
          "`orientation='vertical'` stacks the list down the left edge and aligns triggers to the start.",
      },
    },
  },
  render: () => (
    <Tabs
      defaultValue={panels[0].value}
      orientation="vertical"
      className="w-lg flex-row items-start gap-4"
    >
      <TabsList>
        {panels.map(panel => (
          <TabsTrigger key={panel.value} value={panel.value}>
            {panel.label}
          </TabsTrigger>
        ))}
      </TabsList>
      {panels.map(panel => (
        <TabsContent key={panel.value} value={panel.value}>
          <p className="text-sm text-muted-foreground">{panel.body}</p>
        </TabsContent>
      ))}
    </Tabs>
  ),
};
