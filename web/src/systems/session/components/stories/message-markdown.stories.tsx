import type { Meta, StoryObj } from "@storybook/react-vite";
import type { ReactNode } from "react";

import { CenteredSurface } from "@/storybook/story-layout";
import { markdownFixture } from "@/systems/session/mocks";

import { MessageMarkdown } from "../message-markdown";

const meta: Meta<typeof MessageMarkdown> = {
  title: "systems/session/components/MessageMarkdown",
  component: MessageMarkdown,
  parameters: {
    layout: "centered",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function MarkdownFrame({ children }: { children: ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full rounded-md border border-line bg-canvas-soft p-6">{children}</div>
    </CenteredSurface>
  );
}

export const Default: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content={markdownFixture} />
    </MarkdownFrame>
  ),
};

const RICH_MARKDOWN = `# Heading one

Body paragraph at card-title size with a [strong link](https://compozy.com) and \`inline code\`.

## Heading two

### Heading three

#### Heading four

##### Heading five

###### Heading six

- Nested list
  - Circle nested
    - Square nested
- Sibling item

1. Ordered one
2. Ordered two

- [ ] Task open
- [x] Task done

> Blockquote with muted tone and a left rule.

| Col A | Col B | Col C |
| ----- | ----- | ----- |
| alpha | beta  | gamma |
| delta | epsilon | zeta |

| Wide | Overflowing | Column | Set | For | Horizontal | Scroll |
| ---- | ----------- | ------ | --- | --- | ----------- | ------ |
| very-long-unbroken-token-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa | value | more | data | here | and | beyond |

---

\`\`\`ts
export function greet(name: string): string {
  return \`hello \${name}\`;
}
\`\`\`

\`\`\`bash
compozy session list --workspace risk
\`\`\`

\`\`\`json
{ "ok": true, "count": 2 }
\`\`\`

\`\`\`
plain one-liner fence
\`\`\`

\`\`\`ts
${Array.from({ length: 24 }, (_, i) => `const line${i} = ${i};`).join("\n")}
\`\`\`

Visit [/agents](/agents) or <https://example.com/very/long/path/that-should-wrap-gracefully-across-the-column>.

Final paragraph with **strong** emphasis.
`;

export const RichContent: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content={RICH_MARKDOWN} />
    </MarkdownFrame>
  ),
};

const LONG_ANSWER_MARKDOWN = `I read \`internal/session/store.go\` and the last three incident notes. Short version: keep SQLite, move transcript bodies out of the hot table, and gate it behind a migration. Background on the write path is in the [SQLite WAL documentation](https://www.sqlite.org/wal.html).

# Session transcript persistence

Transcripts are append-heavy and read rarely, but today every message lands in the same table the thread list queries. That is the source of the **p95 regression on workspace open**, not the daemon restart you suspected.

## Options compared

Three candidates survived. Latency figures come from the 40k-message fixture on an M2, cold cache.

| Option | Write path | List p95 | Migration cost | Verdict |
| ------ | ---------- | -------- | -------------- | ------- |
| Status quo | Single \`messages\` table | 412 ms | None | Too slow past 25k rows |
| Split body table | \`messages\` + \`message_bodies\` | 38 ms | One forward migration, lossless | **Ship first** |
| Per-session files | JSONL under the workspace dir | 31 ms | Export, backfill, and new GC rules | Revisit after 0.4 |
| External store | Postgres via adapter | 55 ms | New dependency and config keys | Out of scope |

## Recommendation

Split the body table. It gets 90% of the win, keeps a single file for backups, and the compatibility policy is easy to satisfy because the shape change is internal to SQLite. See [the compatibility regimes](https://github.com/compozy/compozy/blob/main/AGENTS.md) for why that matters.

### Rollout steps

1. Add \`message_bodies\` with a foreign key to \`messages.id\`.
   - Backfill in batches of 500 inside one transaction per batch.
   - Keep the old column until the backfill reports zero remaining rows.
     - Verify with a row-count parity check, not a sample.
2. Switch the thread list query to select from \`messages\` only.
3. Drop the old column in the following release.

### Risks

- A crash mid-backfill leaves mixed rows. The reader must accept both shapes for one release.
- Export tooling reads the old column directly and needs the same change.

> User state upgrades losslessly; every shape change ships its migration.

#### Rollback

The old column is still populated during the window, so rollback is a query change, not a data restore.

\`\`\`sql
-- parity check before dropping the legacy column
SELECT count(*) FROM messages m
LEFT JOIN message_bodies b ON b.message_id = m.id
WHERE b.message_id IS NULL AND m.body IS NOT NULL;
\`\`\`

---

##### Open questions

- Should attachments move too?
- Does the desktop build share the same migration runner?
`;

export const LongAnswer: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content={LONG_ANSWER_MARKDOWN} />
    </MarkdownFrame>
  ),
};

export const NarrowColumn: Story = {
  render: () => (
    <MarkdownFrame>
      <div className="w-88 max-w-full">
        <MessageMarkdown content={LONG_ANSWER_MARKDOWN} />
      </div>
    </MarkdownFrame>
  ),
};

export const LongUnbrokenToken: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content="Wrap check: abcdefghijklmnopqrstuvwxyz0123456789_ABCDEFGHIJKLMNOPQRSTUVWXYZ_token_should_break_inside_the_thread_column_without_horizontal_page_scroll." />
    </MarkdownFrame>
  ),
};
