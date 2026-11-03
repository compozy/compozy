import type { Meta, StoryObj } from "@storybook/react-vite";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import type { CompozyShellBridge } from "@/systems/os/lib/desktop-shell-bridge";
import { workspaceKeys } from "@/systems/workspace/lib/query-keys";

import { CenteredSurface } from "@/storybook/story-layout";
import { markdownFixture } from "@/systems/session/mocks";

import { sessionKeys } from "../../lib/query-keys";
import { SessionRuntimeRenderProvider } from "../../lib/session-runtime-render-context";
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
      <div className="w-full rounded-lg bg-canvas p-6 shadow-card">{children}</div>
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

Ship it in three steps, each behind its own release:

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

export const Compact: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content={LONG_ANSWER_MARKDOWN} compact />
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

export const HostImageReference: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content="![Desktop preview](/tmp/render/desktop-idle.png)" />
    </MarkdownFrame>
  ),
};

const PATHS_WORKSPACE = "ws_story";
const PATHS_SESSION = "sess-story";
const openInEditor = fn(async () => ({ status: "opened" as const }));

function pathsQueryClient() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(sessionKeys.detail(PATHS_WORKSPACE, PATHS_SESSION), {
    id: PATHS_SESSION,
    workspace_id: PATHS_WORKSPACE,
    working_dir: "/Users/dora/compozy",
  });
  client.setQueryData(workspaceKeys.detail(PATHS_WORKSPACE), {
    workspace: { id: PATHS_WORKSPACE, root_dir: "/Users/dora/compozy" },
  });
  return client;
}

const FILE_PATHS_MARKDOWN = `I changed \`internal/mcp/hosted_proxy.go:42\` and \`web/src/app.tsx\`.

The config lives in [your config file](~/.compozy/config.toml); the report is at [report](/tmp/report.md).
Docs: [compozy.com](https://compozy.com). \`compozy start\` is not a path.`;

/** Desktop app (spec #1167): path inline code and links open in the editor on ⌘-click. */
export const DesktopFilePaths: Story = {
  beforeEach: () => {
    window.compozyShell = {
      platform: "darwin",
      files: { openInEditor },
    } as unknown as CompozyShellBridge;
    return () => {
      delete window.compozyShell;
    };
  },
  render: () => (
    <QueryClientProvider client={pathsQueryClient()}>
      <SessionRuntimeRenderProvider sessionId={PATHS_SESSION} workspaceId={PATHS_WORKSPACE}>
        <MarkdownFrame>
          <MessageMarkdown openPaths content={FILE_PATHS_MARKDOWN} />
        </MarkdownFrame>
      </SessionRuntimeRenderProvider>
    </QueryClientProvider>
  ),
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByRole("button", { name: "Open hosted_proxy.go in editor" }));
    await waitFor(() =>
      expect(openInEditor).toHaveBeenCalledWith({
        path: "/Users/dora/compozy/internal/mcp/hosted_proxy.go",
        line: 42,
        roots: ["/Users/dora/compozy"],
      })
    );
    await expect(canvas.queryByRole("button", { name: /compozy start/ })).toBeNull();
  },
};
