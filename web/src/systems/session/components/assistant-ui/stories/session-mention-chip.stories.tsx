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
    const chip = canvas.getByTestId("session-mention-chip");
    await expect(chip).toHaveTextContent("My Notes.md");
    // Screen readers get the full path as hidden text inside the chip.
    await expect(within(chip).getByText("File docs/My Notes.md")).toBeInTheDocument();
  },
};

/**
 * VC-14 — hovering a chip opens the `@compozy/ui` Tooltip with the full path
 * relative to the working directory; a long unspaced path wraps.
 */
export const TooltipOpen: Story = {
  args: {
    kind: "file",
    target: "web/src/components/assistant-ui/session-composer-trigger-menu-classes.ts",
    label: "session-composer-trigger-menu-classes.ts",
  },
  render: args => (
    <p className="max-w-md pt-16 text-small-body leading-relaxed text-fg">
      Review <SessionMentionChip {...args} /> first
    </p>
  ),
  play: async ({ canvasElement, args }) => {
    await userEvent.hover(within(canvasElement).getByTestId("session-mention-chip"));
    const tooltip = await within(canvasElement.ownerDocument.body).findByText(args.target, {
      selector: "[data-slot='tooltip-content']",
    });
    await expect(tooltip).toBeVisible();
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
        <div className="w-md rounded-xl bg-chat-fill-user px-3.5 py-2 text-transcript-message leading-relaxed text-fg">
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

/**
 * VC-21 — the session was renamed to "ACP contract — final" after the send;
 * the chip keeps the title recorded in the message's `mentions` metadata.
 */
export const RenamedSession: Story = {
  args: { kind: "session", target: "sess-7f9e21", label: "Review ACP prompt contract" },
  render: function RenamedSessionStory(args) {
    return (
      <SubagentNavigationContext value={{ openSession: fn() }}>
        <div className="w-md rounded-xl bg-chat-fill-user px-3.5 py-2 text-transcript-message leading-relaxed text-fg">
          <SessionDirectiveText
            text="What did @session:sess-7f9e21 conclude?"
            directives={[]}
            mentions={[
              {
                kind: args.kind,
                target: args.target,
                label: args.label,
                token: "@session:sess-7f9e21",
                start: 9,
                end: 29,
              },
            ]}
          />
        </div>
      </SubagentNavigationContext>
    );
  },
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).getByTestId("session-mention-chip")).toHaveTextContent(
      "Review ACP prompt contract"
    );
  },
};

/**
 * VC-22 — a message sent before the feature has no `mentions` metadata and
 * renders exactly as before, even where the text looks like a path.
 */
export const PreFeatureMessage: Story = {
  args: { kind: "file", target: "internal/acp/client.go", label: "client.go" },
  render: () => (
    <div className="w-md rounded-xl bg-chat-fill-user px-3.5 py-2 text-transcript-message leading-relaxed text-fg">
      <SessionDirectiveText
        text="It started after we touched the @Override handler in internal/acp/client.go"
        directives={[]}
      />
    </div>
  ),
  play: async ({ canvasElement }) => {
    await expect(within(canvasElement).queryByTestId("session-mention-chip")).toBeNull();
  },
};
