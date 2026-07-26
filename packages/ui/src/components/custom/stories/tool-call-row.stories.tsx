import type { Meta, StoryObj } from "@storybook/react-vite";

import { CodeBlock } from "../code-block";
import { ToolCallRow, type ToolCallStatus } from "../tool-call-row";

const meta: Meta<typeof ToolCallRow> = {
  title: "components/custom/ToolCallRow",
  component: ToolCallRow,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Single-line tool call in the transcript language. One ~26px row: icon well + tool heading + mono preview + expand chevron + signal-toned status glyph. Expandable rows open an inline indented body (Input/Output) on click or Enter/Space; hover uses the neutral glaze, never an accent fill.",
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

const INPUT_JSON = `{
  "path": "internal/api/handlers/sessions.go",
  "encoding": "utf-8"
}`;

const SHORT_OUTPUT = `package handlers

func ListSessions(w http.ResponseWriter, r *http.Request) { /* … */ }`;

const LONG_OUTPUT = Array.from({ length: 240 }, (_, index) => `line ${index + 1}`).join("\n");

const STATUSES: ToolCallStatus[] = ["pending", "running", "failed", "success", "empty"];

export const Running: Story = {
  args: {
    toolName: "Bash",
    preview: "agh agent validate ./agent.toml",
    status: "running",
  },
};

export const Done: Story = {
  args: {
    toolName: "Read",
    preview: "internal/api/handlers/sessions.go",
    status: "success",
  },
};

export const FailureWithError: Story = {
  args: {
    toolName: "Read",
    preview: "internal/api/handlers/sessions.go",
    status: "failed",
    errorMessage: "ENOENT: no such file or directory",
  },
};

export const Expandable: Story = {
  args: {
    toolName: "Read",
    preview: "internal/api/handlers/sessions.go",
    status: "success",
  },
  render: args => (
    <ToolCallRow {...args}>
      <ToolCallRow.Input source={INPUT_JSON} format="code" language="json" />
      <ToolCallRow.Output source={SHORT_OUTPUT} format="code" language="go" />
    </ToolCallRow>
  ),
};

export const ExpandedByDefault: Story = {
  args: {
    toolName: "Read",
    preview: "internal/api/handlers/sessions.go",
    status: "success",
    defaultExpanded: true,
  },
  render: args => (
    <ToolCallRow {...args}>
      <ToolCallRow.Input source={INPUT_JSON} format="code" language="json" />
      <ToolCallRow.Output source={SHORT_OUTPUT} format="code" language="go" />
    </ToolCallRow>
  ),
};

export const LargeOutputScrolls: Story = {
  args: {
    toolName: "Read",
    preview: "internal/api/handlers/sessions.go",
    status: "success",
    defaultExpanded: true,
  },
  render: args => (
    <ToolCallRow {...args}>
      <ToolCallRow.Input source={INPUT_JSON} format="code" language="json" />
      <ToolCallRow.Output source={LONG_OUTPUT} format="code" language="plaintext" />
    </ToolCallRow>
  ),
};

export const MarkdownInput: Story = {
  args: {
    toolName: "review.summarize",
    status: "success",
    defaultExpanded: true,
  },
  render: args => (
    <ToolCallRow {...args}>
      <ToolCallRow.Input
        format="markdown"
        source={[
          "Summarize **only** the failing checks and ignore passing rows.",
          "",
          "- Highlight regressions vs main",
          "- Suggest a one-line revert candidate",
        ].join("\n")}
      />
      <ToolCallRow.Output
        format="markdown"
        source="_No regressions vs `main` — all 12 checks green._"
      />
    </ToolCallRow>
  ),
};

/**
 * A shiki-highlighted `<CodeBlock>` mounted directly as children — mirrors the
 * web session adapter's explicit-CodeBlock composition through the body slot.
 */
export const CodeBlockChild: Story = {
  args: {
    toolName: "Read",
    preview: "internal/api/handlers/sessions.go",
    status: "success",
    defaultExpanded: true,
  },
  render: args => (
    <ToolCallRow {...args}>
      <ToolCallRow.Input>
        <CodeBlock language="json" code={INPUT_JSON} density="compact" />
      </ToolCallRow.Input>
    </ToolCallRow>
  ),
};

const LONG_PREVIEW =
  'agh tool invoke agh__tool_info --input \'{"tool_id":"agh__skill_view","workspace_id":"ws-demo"}\' -o json';

export const LongPreviewTruncates: Story = {
  args: {
    toolName: "Bash",
    preview: LONG_PREVIEW,
    status: "success",
  },
  render: args => (
    <ToolCallRow {...args}>
      <ToolCallRow.Input source={LONG_PREVIEW} format="code" language="bash" />
      <ToolCallRow.Output source="ok" format="code" language="plaintext" />
    </ToolCallRow>
  ),
};

export const AllStatuses: Story = {
  render: () => (
    <div className="flex flex-col gap-0.5" data-testid="all-statuses">
      {STATUSES.map(status => (
        <ToolCallRow
          key={status}
          toolName="Read"
          preview="packages/runtime/src/session/stream.ts"
          status={status}
          data-status-key={status}
        />
      ))}
    </div>
  ),
};

/**
 * The density win: a whole tool-heavy turn as a stack of ~26px lines instead of
 * a wall of 44px cards.
 */
export const LiveStack: Story = {
  render: () => (
    <div className="flex flex-col gap-0.5" data-testid="live-stack">
      <ToolCallRow toolName="Bash" preview="agh agent validate ./agent.toml" status="success">
        <ToolCallRow.Output
          source="✓ schema valid\n✓ tools resolved (4)"
          format="code"
          language="plaintext"
        />
      </ToolCallRow>
      <ToolCallRow toolName="Read" preview="agh.config.toml" status="success">
        <ToolCallRow.Output source={"[runtime]\nmode = local"} format="code" language="toml" />
      </ToolCallRow>
      <ToolCallRow toolName="Grep" preview='pattern: "INPUT|OUTPUT"' status="success">
        <ToolCallRow.Output
          source="internal/api/core/handlers.go"
          format="code"
          language="plaintext"
        />
      </ToolCallRow>
      <ToolCallRow toolName="Write" preview="internal/api/core/session_recap.go" status="running" />
      <ToolCallRow
        toolName="Read"
        preview="internal/store/sessiondb/session_db.go"
        status="pending"
      />
    </div>
  ),
};
