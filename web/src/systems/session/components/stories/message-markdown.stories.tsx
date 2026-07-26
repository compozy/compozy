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

Body paragraph at card-title size with a [strong link](https://agh.network) and \`inline code\`.

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
agh session list --workspace risk
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

Final paragraph with **medium strong** emphasis.
`;

export const RichContent: Story = {
  render: () => (
    <MarkdownFrame>
      <MessageMarkdown content={RICH_MARKDOWN} />
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
