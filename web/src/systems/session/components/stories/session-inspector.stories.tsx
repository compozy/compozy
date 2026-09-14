import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { Button } from "@compozy/ui";
import { PanelSurface } from "@/storybook/story-layout";
import { SessionInspector } from "../session-inspector";
import { deriveSessionContext } from "../../lib/session-context";
import { sessionContextFixture, sessionContextTurnsFixture } from "../../mocks/context-fixtures";

const meta = {
  title: "systems/session/components/SessionInspector",
  component: SessionInspector,
  parameters: { layout: "fullscreen" },
  decorators: [
    Story => (
      <PanelSurface className="min-h-160 justify-end">
        <Story />
      </PanelSurface>
    ),
  ],
  args: {
    context: deriveSessionContext(sessionContextFixture),
    turns: sessionContextTurnsFixture,
    usage: {
      tokensIn: 128_400,
      tokensOut: 24_900,
      totalTokens: 153_300,
      cacheReadTokens: 102_400,
      cacheWriteTokens: 12_800,
      costUsd: 18.42,
      costCurrency: "USD",
      costStatus: "actual",
      costSource: "agent_reported",
      turnCount: 12,
    },
    activity: {
      status: "Working for 49m 20s · Running shell",
      agents: "2 agents running",
      tools: "38 tools",
      thoughts: "12 thoughts",
      queued: "1 queued",
      goal: "Goal · turn 0/20 · active",
    },
  },
} satisfies Meta<typeof SessionInspector>;
export default meta;
type Story = StoryObj<typeof meta>;
export const Full: Story = {};
export const EstimateExceeds: Story = {
  args: {
    context: deriveSessionContext({
      ...sessionContextFixture,
      used: 225_280,
      ratio: 0.88,
      injected: { ...sessionContextFixture.injected!, tokens: 231_000 },
    }),
  },
};
export const OverCapacity: Story = {
  args: { context: deriveSessionContext({ ...sessionContextFixture, used: 281_600, ratio: 1.1 }) },
};
export const RowsExpanded: Story = { args: { injectedDefaultOpen: true } };
export const UnknownWithRows: Story = {
  args: {
    context: deriveSessionContext({ state: "unknown", injected: sessionContextFixture.injected }),
    injectedDefaultOpen: true,
  },
};
export const Summarized: Story = {
  args: {
    context: deriveSessionContext({
      ...sessionContextFixture,
      stale: true,
      injected: {
        ...sessionContextFixture.injected!,
        stale: true,
        rows: sessionContextFixture.injected!.rows.map(row => ({ ...row, stale: true })),
      },
    }),
    injectedDefaultOpen: true,
  },
};
export const Unavailable: Story = {
  args: { context: deriveSessionContext(sessionContextFixture, { unavailable: true }) },
};
export const TokensWithCache: Story = {};
export const TurnsUnion: Story = {
  args: {
    turns: {
      ...sessionContextTurnsFixture,
      turns: sessionContextTurnsFixture.turns.map(turn =>
        turn.turn_id === "turn-2"
          ? { ...turn, usage: { ...turn.usage!, cost_amount: 0.03, cost_currency: "USD" } }
          : turn
      ),
    },
  },
};
export const ManyTurns: Story = {
  args: {
    turns: {
      compactions: [],
      turns: Array.from({ length: 120 }, (_, index) => ({
        turn_id: `turn-${index + 1}`,
        sequence: index + 1,
        usage: { input_tokens: 1_000, timestamp: "2026-09-12T10:00:00Z" },
      })),
    },
  },
};
export const OpaqueTurnIds: Story = {
  args: {
    turns: {
      compactions: [],
      turns: [
        {
          turn_id: "turn-ea76af9f25086e7d",
          sequence: 121,
          usage: {
            context_used: 89_690,
            context_size: 256_000,
            input_tokens: 1_200,
            output_tokens: 240,
            cost_amount: 0.03,
            cost_currency: "USD",
            timestamp: "2026-09-12T10:00:00Z",
          },
        },
        {
          turn_id: "turn-120",
          sequence: 120,
          usage: { input_tokens: 1_000, timestamp: "2026-09-12T10:00:00Z" },
        },
      ],
    },
  },
};
export const ActivityStopped: Story = {
  args: {
    activity: {
      status: "Stopped by you after 1h 12m",
      tools: "38 tools",
      thoughts: "12 thoughts",
    },
  },
};
export const ActivityLive: Story = {
  args: {
    activity: { ...meta.args.activity, warning: "Runtime warning · provider slow to respond" },
  },
};
export const Empty: Story = {
  args: {
    context: deriveSessionContext(),
    usage: null,
    turns: { turns: [], compactions: [] },
    activity: {},
  },
};
export const Drawer: Story = {
  render: function DrawerStory(args) {
    const [open, setOpen] = useState(true);
    return (
      <>
        <Button variant="neutral" onClick={() => setOpen(true)}>
          Open context sidebar
        </Button>
        <SessionInspector {...args} drawerOpen={open} onDrawerOpenChange={setOpen} />
      </>
    );
  },
  parameters: { viewport: { defaultViewport: "tablet" } },
};

export const Warning: Story = {
  args: { context: deriveSessionContext({ ...sessionContextFixture, used: 225_280, ratio: 0.88 }) },
};
export const IncludedWithoutCache: Story = {
  args: {
    usage: {
      tokensIn: 18_410,
      tokensOut: 2_102,
      totalTokens: 20_512,
      costStatus: "included",
      costSource: "none",
      turnCount: 3,
    },
  },
};
