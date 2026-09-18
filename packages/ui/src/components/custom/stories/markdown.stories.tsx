import type { Meta, StoryObj } from "@storybook/react-vite";

import { Markdown } from "../markdown";

const meta: Meta<typeof Markdown> = {
  title: "components/custom/Markdown",
  component: Markdown,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          'Prose grammar for every rendered Markdown surface. The default density is the reading column; `compact` keeps the small heading tier for dense panels, and `compact="relaxed"` pairs that tier with prose paragraph breaks.',
      },
    },
  },
  decorators: [
    Story => (
      <div className="w-[720px] bg-background p-6">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof meta>;

const PROSE = [
  "Keep SQLite and move transcript bodies out of the hot table. Background is in the [SQLite WAL documentation](https://www.sqlite.org/wal.html).",
  "",
  "# Session transcript persistence",
  "",
  "Transcripts are append-heavy and read rarely. That is the source of the **p95 regression on workspace open**.",
  "",
  "## Options compared",
  "",
  "| Option | Write path | List p95 | Verdict |",
  "| ------ | ---------- | -------- | ------- |",
  "| Status quo | Single `messages` table | 412 ms | Too slow past 25k rows |",
  "| Split body table | `messages` + `message_bodies` | 38 ms | **Ship first** |",
  "",
  "### Rollout steps",
  "",
  "Ship it in three steps:",
  "",
  "1. Add `message_bodies` with a foreign key.",
  "   - Backfill in batches of 500.",
  "     - Verify with a row-count parity check.",
  "2. Switch the thread list query.",
  "",
  "> User state upgrades losslessly; every shape change ships its migration.",
  "",
  "#### Rollback",
  "",
  "The old column is still populated during the window.",
  "",
  "##### Open questions",
  "",
  "- Should attachments move too?",
].join("\n");

export const Reading: Story = {
  render: () => <Markdown>{PROSE}</Markdown>,
};

export const Compact: Story = {
  render: () => <Markdown compact>{PROSE}</Markdown>,
};

export const CompactRelaxed: Story = {
  render: () => <Markdown compact="relaxed">{PROSE}</Markdown>,
};
