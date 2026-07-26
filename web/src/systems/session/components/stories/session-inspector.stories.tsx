import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { fn, userEvent, within } from "storybook/test";

import { Button } from "@agh/ui";

import { PanelSurface } from "@/storybook/story-layout";
import type { VaultSecret } from "@/systems/vault/types";

import { SessionInspector, type SessionInspectorProps } from "../session-inspector";

const vaultSecrets: VaultSecret[] = [
  {
    ref: "vault:sessions/session_launch_coordination/stripe_api_key",
    namespace: "sessions",
    kind: "api_key",
    present: true,
    created_at: "2026-04-17T18:00:00Z",
    updated_at: "2026-04-17T18:14:00Z",
  },
];

const baseArgs: SessionInspectorProps = {
  messages: [],
  sessionId: "session_launch_coordination",
  usage: {
    tokensIn: 128_400,
    tokensOut: 24_900,
    totalTokens: 153_300,
    costUsd: 18.42,
    costCurrency: "USD",
    costStatus: "actual",
    costSource: "agent_reported",
    turnCount: 12,
  },
  vaultSecrets,
  files: [
    { path: "web/src/systems/session/components/session-inspector.tsx", readCount: 3 },
    { path: "internal/session/runtime.go", readCount: 1 },
  ],
  onViewAllTrace: fn(),
};

const meta: Meta<typeof SessionInspector> = {
  title: "systems/session/components/SessionInspector",
  component: SessionInspector,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Right-side session inspector consuming <DetailInspector>: inline at >= 1440 px viewport, drawer below.",
      },
    },
  },
  decorators: [
    Story => (
      <PanelSurface className="min-h-[640px] justify-end">
        <Story />
      </PanelSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Inline inspector at >= 1440 px viewport renders 5 flat tabs
 * (Trace, Usage, Memory, Files, Vault) inside the DetailInspector chrome.
 */
export const Inline: Story = {
  args: baseArgs,
  parameters: {
    viewport: { defaultViewport: "responsive" },
  },
};

/**
 * Drawer variant exposes the same body inside the sheet drawer when the
 * viewport is < 1440 px. Caller owns the open state.
 */
export const Drawer: Story = {
  args: baseArgs,
  render: function DrawerStory(args) {
    const [open, setOpen] = useState(false);
    return (
      <div className="flex h-full w-full items-start justify-end gap-2 p-4">
        <Button type="button" variant="neutral" size="sm" onClick={() => setOpen(true)}>
          Open inspector
        </Button>
        <SessionInspector {...args} drawerOpen={open} onDrawerOpenChange={setOpen} />
      </div>
    );
  },
};

/**
 * Usage tab wired to a real daemon token-usage summary: tokens in/out, total
 * tokens, and cost render as a truthful metric grid with a turn-count caption.
 */
export const UsageTab: Story = {
  args: baseArgs,
  parameters: {
    viewport: { defaultViewport: "responsive" },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("session-inspector-tab-usage"));
    await canvas.findByTestId("session-inspector-usage-grid");
  },
};

/**
 * Estimated cost: catalog projection rendered with the ≈ cue and an "Estimated"
 * provenance line, so it never reads as measured spend.
 */
export const UsageEstimated: Story = {
  args: {
    ...baseArgs,
    usage: {
      tokensIn: 128_400,
      tokensOut: 24_900,
      totalTokens: 153_300,
      costUsd: 0.42,
      costCurrency: "USD",
      costStatus: "estimated",
      costSource: "catalog_config",
      turnCount: 12,
    },
  },
  parameters: {
    viewport: { defaultViewport: "responsive" },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("session-inspector-tab-usage"));
    await canvas.findByTestId("session-inspector-usage-grid");
  },
};

/**
 * Subscription-included usage: token counts stay visible while the cost cell
 * shows "Included" with no monetary amount.
 */
export const UsageIncluded: Story = {
  args: {
    ...baseArgs,
    usage: {
      tokensIn: 128_400,
      tokensOut: 24_900,
      totalTokens: 153_300,
      costStatus: "included",
      costSource: "none",
      turnCount: 12,
    },
  },
  parameters: {
    viewport: { defaultViewport: "responsive" },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("session-inspector-tab-usage"));
    await canvas.findByTestId("session-inspector-usage-grid");
  },
};

/**
 * Unknown cost: the daemon reports usage but no cost, so the cell reads
 * "Unavailable" with no monetary amount.
 */
export const UsageUnknown: Story = {
  args: {
    ...baseArgs,
    usage: {
      tokensIn: 128_400,
      tokensOut: 24_900,
      totalTokens: 153_300,
      costStatus: "unknown",
      turnCount: 12,
    },
  },
  parameters: {
    viewport: { defaultViewport: "responsive" },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("session-inspector-tab-usage"));
    await canvas.findByTestId("session-inspector-usage-grid");
  },
};

/**
 * Classification-only usage: the daemon reports a subscription cost status with no
 * token counters. The panel still opens — cost reads "Included", every token metric
 * shows "—" — proving presence follows authoritative status, not token counts.
 */
export const UsageClassificationOnly: Story = {
  args: {
    ...baseArgs,
    usage: {
      costStatus: "included",
      costSource: "none",
      turnCount: 0,
    },
  },
  parameters: {
    viewport: { defaultViewport: "responsive" },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("session-inspector-tab-usage"));
    await canvas.findByTestId("session-inspector-usage-grid");
  },
};

/**
 * Empty inspector keeps each tab truthful before the first turn completes.
 */
export const Empty: Story = {
  args: {
    messages: [],
    sessionId: "session_empty",
    usage: null,
    vaultSecrets: [],
    files: [],
    onViewAllTrace: fn(),
  },
};
