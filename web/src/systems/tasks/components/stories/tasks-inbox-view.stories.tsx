import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { PanelSurface } from "@/storybook/story-layout";
import type { InboxLaneFilter } from "../../hooks/use-tasks-page";
import type { TaskInboxView, TaskPriority, TaskStatus } from "../../types";
import { TasksInboxView } from "../tasks-inbox-view";
import { buildInboxFixture, buildInboxItemFixture } from "../test-fixtures";

const meta: Meta<typeof TasksInboxView> = {
  title: "systems/tasks/components/TasksInboxView",
  component: TasksInboxView,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Actor-scoped task inbox with exact lane counts, server-owned filters, and cursor continuation.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <PanelSurface className="min-h-[720px] p-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{children}</div>
    </PanelSurface>
  );
}

const POPULATED: TaskInboxView = buildInboxFixture({
  archived_total: 0,
  facets: {
    priorities: [{ count: 5, priority: "medium" }],
    statuses: [
      { count: 1, status: "completed" },
      { count: 1, status: "failed" },
      { count: 2, status: "pending" },
      { count: 1, status: "ready" },
    ],
  },
  page: { has_more: false, limit: 50, total: 5 },
  unread_total: 3,
  groups: [
    {
      lane: "approvals",
      count: 2,
      unread_count: 2,
      items: [
        buildInboxItemFixture({
          lane: "approvals",
          approval_policy: "manual",
          approval_state: "pending",
          task: {
            id: "task_i1",
            identifier: "TASK-101",
            scope: "workspace",
            status: "pending",
            title: "Approve: edit /etc/hosts",
            owner: { kind: "agent_session", ref: "claude" },
          },
          triage: {
            actor: { kind: "human", ref: "op" },
            archived: false,
            dismissed: false,
            read: false,
            task_id: "task_i1",
            updated_at: "2026-04-17T10:00:00Z",
          },
        }),
        buildInboxItemFixture({
          lane: "approvals",
          approval_policy: "manual",
          approval_state: "pending",
          task: {
            id: "task_i2",
            identifier: "TASK-102",
            scope: "workspace",
            status: "pending",
            title: "Approve: deploy to prod",
            owner: { kind: "agent_session", ref: "codex" },
          },
          triage: {
            actor: { kind: "human", ref: "op" },
            archived: false,
            dismissed: false,
            read: false,
            task_id: "task_i2",
            updated_at: "2026-04-17T09:40:00Z",
          },
        }),
      ],
    },
    {
      lane: "failed_runs",
      count: 1,
      unread_count: 1,
      items: [
        buildInboxItemFixture({
          lane: "failed_runs",
          task: {
            id: "task_i3",
            identifier: "TASK-103",
            scope: "workspace",
            status: "failed",
            title: "Bridge delivery failed -- slack#alerts",
            owner: { kind: "agent_session", ref: "codex" },
          },
          run: {
            attempt: 2,
            id: "run_i3",
            max_attempts: 3,
            queued_at: "2026-04-17T09:22:00Z",
            recovery_count: 0,
            status: "failed",
            error: "unexpected 502 from upstream",
            task_id: "task_i3",
          },
          triage: {
            actor: { kind: "human", ref: "op" },
            archived: false,
            dismissed: false,
            read: false,
            task_id: "task_i3",
            updated_at: "2026-04-17T09:25:00Z",
          },
        }),
      ],
    },
    {
      lane: "my_work",
      count: 2,
      unread_count: 0,
      items: [
        buildInboxItemFixture({
          task: {
            id: "task_i4",
            identifier: "TASK-104",
            scope: "workspace",
            status: "ready",
            title: "Skill installed: git-flow-guard",
            owner: { kind: "agent_session", ref: "claude" },
          },
          triage: {
            actor: { kind: "human", ref: "op" },
            archived: false,
            dismissed: false,
            read: true,
            task_id: "task_i4",
            updated_at: "2026-04-17T08:10:00Z",
          },
        }),
        buildInboxItemFixture({
          task: {
            id: "task_i5",
            identifier: "TASK-105",
            scope: "workspace",
            status: "completed",
            title: "Retry succeeded: memory GC policy",
            owner: { kind: "agent_session", ref: "claude" },
          },
          triage: {
            actor: { kind: "human", ref: "op" },
            archived: false,
            dismissed: false,
            read: true,
            task_id: "task_i5",
            updated_at: "2026-04-17T05:40:00Z",
          },
        }),
      ],
    },
  ],
});

function ControlledInbox(props: Partial<Parameters<typeof TasksInboxView>[0]>) {
  const [lane, setLane] = useState<InboxLaneFilter>("all");
  const [status, setStatus] = useState<TaskStatus | null>(null);
  const [priority, setPriority] = useState<TaskPriority | null>(null);
  const [unreadOnly, setUnreadOnly] = useState(false);
  const [query, setQuery] = useState("");
  return (
    <TasksInboxView
      inbox={POPULATED}
      laneFilter={lane}
      onLaneChange={setLane}
      onPriorityChange={setPriority}
      onSearchChange={setQuery}
      onStatusChange={setStatus}
      onToggleUnread={setUnreadOnly}
      priorityFilter={priority}
      searchQuery={query}
      statusFilter={status}
      unreadOnly={unreadOnly}
      {...props}
    />
  );
}

export const Populated: Story = {
  args: {},
  render: () => (
    <Frame>
      <ControlledInbox />
    </Frame>
  ),
};

export const EmptyInbox: Story = {
  args: {},
  name: "Empty",
  render: () => (
    <Frame>
      <ControlledInbox inbox={buildInboxFixture()} />
    </Frame>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <Frame>
      <ControlledInbox inbox={null} isLoading />
    </Frame>
  ),
};

export const ErrorState: Story = {
  args: {},
  name: "Error",
  render: () => (
    <Frame>
      <ControlledInbox errorMessage="Inbox unavailable" inbox={null} />
    </Frame>
  ),
};
