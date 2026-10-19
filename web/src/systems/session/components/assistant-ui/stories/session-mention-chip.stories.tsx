import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { SessionMentionChip } from "@/components/assistant-ui/session-composer-mention-chip";
import { SessionDirectiveText } from "@/components/assistant-ui/session-directive-text";
import { SubagentNavigationContext } from "@/systems/session/contexts/session-subagents-context-value";

/**
 * Mention chips (S2, S4): the skill chip's text-weight anatomy in neutral ink.
 * The icon carries the kind; the tooltip and accessible name carry the full
 * relative path or session id. In history only daemon-verified `mentions`
 * render as chips, and only a session chip is interactive.
 */
const meta = {
  title: "systems/session/components/assistant-ui/SessionMentionChip",
  component: SessionMentionChip,
  parameters: { layout: "centered" },
} satisfies Meta<typeof SessionMentionChip>;

export default meta;

type Story = StoryObj<typeof meta>;

/** VC-11..VC-13 — file, session and folder chips inline with prompt text. */
export const Kinds: Story = {
  args: { kind: "file", target: "internal/acp/client_prompt.go", label: "client_prompt.go" },
  render: args => (
    <p className="max-w-md text-small-body leading-relaxed text-fg">
      Compare <SessionMentionChip {...args} /> with what{" "}
      <SessionMentionChip kind="session" target="sess-1a2b3c" label="Fix login bug" /> found under{" "}
      <SessionMentionChip kind="folder" target="web/src/" label="src/" />
    </p>
  ),
};

/** VC-15 — a quoted path still shows the plain name; the tooltip names the full path. */
export const QuotedPath: Story = {
  args: { kind: "file", target: "docs/My Notes.md", label: "My Notes.md" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await expect(canvas.getByLabelText("File docs/My Notes.md")).toHaveTextContent("My Notes.md");
  },
};

const historyText =
  "Compare @internal/acp/client_prompt.go with @session:sess-1a2b3c under @web/src/, not @scope/pkg";
const historyMentions = [
  {
    kind: "file" as const,
    token: "@internal/acp/client_prompt.go",
    target: "internal/acp/client_prompt.go",
    label: "client_prompt.go",
    start: 8,
    end: 38,
  },
  {
    kind: "session" as const,
    token: "@session:sess-1a2b3c",
    target: "sess-1a2b3c",
    label: "Fix login bug",
    start: 44,
    end: 64,
  },
  {
    kind: "folder" as const,
    token: "@web/src/",
    target: "web/src/",
    label: "src/",
    start: 71,
    end: 80,
  },
];

/**
 * VC-18..VC-20 — verified mentions in a sent message; the unresolved
 * `@scope/pkg` stays text, and the session chip opens that session.
 */
export const History: Story = {
  args: { kind: "session", target: "sess-1a2b3c", label: "Fix login bug" },
  render: function HistoryStory() {
    const openSession = fn();
    return (
      <SubagentNavigationContext value={{ openSession }}>
        <div className="w-fit max-w-md rounded-xl bg-chat-fill-user px-3.5 py-2 text-transcript-message leading-relaxed text-fg">
          <SessionDirectiveText text={historyText} directives={[]} mentions={historyMentions} />
        </div>
      </SubagentNavigationContext>
    );
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const session = canvas.getByRole("button", { name: "Session Fix login bug (sess-1a2b3c)" });
    await userEvent.hover(session);
    await expect(canvas.getAllByTestId("session-mention-chip")).toHaveLength(3);
  },
};
