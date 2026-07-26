import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { ChatMessageBubble, type ChatMessageRole } from "../chat-message-bubble";
import { Pill } from "../pill";
import { ToolCallRow, type ToolCallStatus } from "../tool-call-row";

const meta: Meta<typeof ChatMessageBubble> = {
  title: "components/custom/ChatMessageBubble",
  component: ChatMessageBubble,
  parameters: {
    layout: "padded",
    docs: {
      description: {
        component:
          "Presentational chat message shell per DESIGN.md §4. Role drives layout: `user` right-aligns with a surface-elevated bubble, `agent` left-aligns without a bubble, `system` renders a full-width hairline row, and `tool`/`diff` are pass-through blocks for composed inline cards.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const ROLES: ChatMessageRole[] = ["user", "agent", "system", "tool", "diff"];

const ROLE_ALIGN: Record<ChatMessageRole, "left" | "right"> = {
  user: "right",
  agent: "left",
  system: "left",
  tool: "left",
  diff: "left",
};

function AgentMeta() {
  return (
    <>
      <Pill.Dot tone="accent" size="sm" />
      <span>claude</span>
      <span className="text-subtle">· 12:03</span>
    </>
  );
}

export const UserRole: Story = {
  args: {
    messageRole: "user",
    meta: "YOU · 12:02",
    children:
      "Find the event mapper that groups tool calls by turn and extract the grouping logic into a pure helper.",
  },
};

export const AgentRole: Story = {
  args: {
    messageRole: "agent",
    meta: <AgentMeta />,
    children:
      "I can see two candidates, `stream.ts` and `map.ts`. I'll extract the grouping into `groupToolCallsByTurn` and point the call site at it.",
  },
};

export const SystemRole: Story = {
  args: {
    messageRole: "system",
    children: "Session resumed from checkpoint 8471 · 3 prior tool calls replayed",
  },
};

export const ToolRole: Story = {
  args: {
    messageRole: "tool",
    children: <ToolCallRow toolName="shell.safe-run" preview="packages/runtime" status="success" />,
  },
};

export const DiffRole: Story = {
  args: {
    messageRole: "diff",
    children: (
      <div className="rounded-md border border-line bg-rail p-4 font-mono text-[12px] leading-[1.65]">
        <div className="text-success">+ const groups = groupToolCallsByTurn(tool.events);</div>
        <div className="text-danger">- for (const ev of tool.events) {"{"}</div>
      </div>
    ),
  },
};

export const AllRoles: Story = {
  render: () => (
    <div
      className="flex flex-col gap-4"
      data-testid="all-roles"
      style={{ maxWidth: 820, margin: "0 auto" }}
    >
      <ChatMessageBubble messageRole="system" data-role-key="system">
        Session resumed · 3 prior tool calls replayed
      </ChatMessageBubble>
      <ChatMessageBubble messageRole="user" meta="YOU · 12:02" data-role-key="user">
        Find the event mapper that groups tool calls by turn.
      </ChatMessageBubble>
      <ChatMessageBubble messageRole="agent" meta={<AgentMeta />} data-role-key="agent">
        Two candidates, I&apos;ll extract the grouping into `groupToolCallsByTurn`.
      </ChatMessageBubble>
      <ChatMessageBubble messageRole="tool" data-role-key="tool">
        <ToolCallRow toolName="shell.safe-run" preview="packages/runtime" status="success" />
      </ChatMessageBubble>
      <ChatMessageBubble messageRole="diff" data-role-key="diff">
        <div className="rounded-md border border-line bg-rail p-3 font-mono text-[12px]">
          + apply diff to stream.ts
        </div>
      </ChatMessageBubble>
    </div>
  ),
};

export const RoleAlignmentInteraction: Story = {
  render: () => (
    <div
      className="flex flex-col gap-3"
      data-testid="role-alignment"
      style={{ maxWidth: 820, margin: "0 auto" }}
    >
      {ROLES.map(role => (
        <ChatMessageBubble
          key={role}
          messageRole={role}
          meta={role === "user" ? "YOU · 12:02" : role === "agent" ? <AgentMeta /> : undefined}
          data-role-key={role}
        >
          {role === "tool" ? (
            <ToolCallRow toolName="shell.run" status="running" />
          ) : (
            `message for role ${role}`
          )}
        </ChatMessageBubble>
      ))}
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const wrapper = await canvas.findByTestId("role-alignment");
    for (const role of ROLES) {
      const node = wrapper.querySelector<HTMLElement>(`[data-role-key="${role}"]`);
      await expect(node).not.toBeNull();
      await expect(node?.getAttribute("data-role")).toBe(role);
      await expect(node?.getAttribute("data-align")).toBe(ROLE_ALIGN[role]);
      if (role === "user") {
        const body = node?.querySelector<HTMLElement>('[data-slot="chat-message-body"]');
        await expect(body?.className).toContain("bg-elevated");
        await expect(node?.className).toContain("justify-end");
      }
      if (role === "agent") {
        const body = node?.querySelector<HTMLElement>('[data-slot="chat-message-body"]');
        await expect(body?.className).not.toContain("bg-elevated");
        await expect(body?.className).toContain("text-muted");
      }
      if (role === "system") {
        const dividers = node?.querySelectorAll('span[aria-hidden="true"]') ?? [];
        await expect(dividers.length).toBe(2);
      }
    }
  },
};

export const StatusBadgeCycleInteraction: Story = {
  render: () => (
    <div
      className="flex flex-col gap-3"
      data-testid="tool-statuses"
      style={{ maxWidth: 820, margin: "0 auto" }}
    >
      {(["pending", "running", "failed", "success", "empty"] as ToolCallStatus[]).map(status => (
        <ChatMessageBubble key={status} messageRole="tool">
          <ToolCallRow
            toolName="file.read"
            preview="packages/runtime/src/session/stream.ts"
            status={status}
            data-status-key={status}
          />
        </ChatMessageBubble>
      ))}
    </div>
  ),
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const wrapper = await canvas.findByTestId("tool-statuses");
    // `pending` is intentionally glyph-less (the row is muted while it prepares
    // input); every resolved/running state carries one signal-toned glyph.
    const glyphLabels: Record<Exclude<ToolCallStatus, "pending">, string> = {
      running: "Running",
      failed: "Error",
      success: "Done",
      empty: "Empty",
    };
    for (const status of ["pending", "running", "failed", "success", "empty"] as ToolCallStatus[]) {
      const card = wrapper.querySelector<HTMLElement>(`[data-status-key="${status}"]`);
      await expect(card).not.toBeNull();
      const badge = card?.querySelector<HTMLElement>('[data-slot="tool-call-row-status"]');
      if (status === "pending") {
        await expect(badge).toBeNull();
        continue;
      }
      await expect(badge?.getAttribute("aria-label")).toBe(
        glyphLabels[status as Exclude<ToolCallStatus, "pending">]
      );
      await expect(badge?.getAttribute("data-status")).toBe(status);
    }
  },
};
