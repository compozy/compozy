import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { CenteredSurface } from "@/storybook/story-layout";

import type { SessionCompactionItemData } from "../../types";
import { SessionCompactionItem } from "../session-compaction-item";

const STARTED_AT = "2026-10-08T14:02:11Z";
const ENDED_AT = "2026-10-08T14:02:39Z";

const SUMMARY = [
  "The billing migration moved invoices and refunds to the ledger API.",
  "",
  "- invoices: cut over, reconciliation job green",
  "- refunds: dual-writing until the backfill finishes",
  "",
  "Next: finish the refund backfill, then remove the legacy writer.",
].join("\n");

function item(overrides: Partial<SessionCompactionItemData>): SessionCompactionItemData {
  return {
    kind: "compaction",
    compaction_id: "c1f0b8f4-6d2e-4a3b-9c1d-0e5f7a8b9c10",
    status: "completed",
    started_at: STARTED_AT,
    ...overrides,
  };
}

const meta: Meta<typeof SessionCompactionItem> = {
  title: "systems/session/components/SessionCompactionItem",
  component: SessionCompactionItem,
  parameters: {
    layout: "centered",
    docs: {
      description: {
        component:
          "One observed agent compaction as a timeline row. `status` is the agent's own word: `in_progress`, `completed`, `failed` and `cancelled` have a sentence of their own; any other value is a vendor status shown verbatim and never treated as finished. The agent's summary, when it sent one, rests behind a closed Summary disclosure.",
      },
    },
  },
  decorators: [
    Story => (
      <CenteredSurface>
        <div className="w-full max-w-2xl rounded-md border border-line bg-canvas py-2 pr-2 pl-1.5">
          <Story />
        </div>
      </CenteredSurface>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

/** Compaction in flight: the spinner glyph of a running tool row; the spin stops under reduced motion. */
export const InProgress: Story = {
  args: { item: item({ status: "in_progress" }) },
};

/** Claude sends a summary: the row carries a closed Summary disclosure. */
export const CompletedWithSummary: Story = {
  args: { item: item({ summary: SUMMARY, ended_at: ENDED_AT }) },
};

/** The summary opened: markdown in the same indented body the reasoning rows use. */
export const SummaryOpen: Story = {
  args: { item: item({ summary: SUMMARY, ended_at: ENDED_AT }) },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Summary" }));
    await expect(canvas.getByTestId("session-compaction-summary-content")).toBeVisible();
  },
};

/** Codex sends no summary: the row is the sentence alone, no disclosure. */
export const CompletedWithoutSummary: Story = {
  args: { item: item({ ended_at: ENDED_AT }) },
};

/** A failed compaction names the agent's error in the sentence. */
export const Failed: Story = {
  args: {
    item: item({
      status: "failed",
      error: "The agent could not summarize the conversation: context window exceeded.",
      ended_at: ENDED_AT,
    }),
  },
};

export const Cancelled: Story = {
  args: { item: item({ status: "cancelled", ended_at: ENDED_AT }) },
};

/** A vendor status passes through verbatim and reads as not finished. */
export const VendorStatus: Story = {
  args: { item: item({ status: "compaction_paused" }) },
};
