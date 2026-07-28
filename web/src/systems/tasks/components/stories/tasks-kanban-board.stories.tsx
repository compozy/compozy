import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";

import { PanelSurface } from "@/storybook/story-layout";
import { countTasksByStatus } from "../../lib/task-formatters";
import { groupTasksForKanban } from "../../lib/task-grouping";
import type { TaskListItem } from "../../types";
import { TasksKanbanBoard } from "../tasks-kanban-board";
import { buildTaskFixture } from "./fixtures";

const meta: Meta<typeof TasksKanbanBoard> = {
  title: "systems/tasks/components/TasksKanbanBoard",
  component: TasksKanbanBoard,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Task status lanes with exact backend facet totals and explicit continuation.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: React.ReactNode }) {
  return (
    <PanelSurface className="min-h-[640px] p-0">
      <div className="flex min-h-0 flex-1 flex-col overflow-hidden">{children}</div>
    </PanelSurface>
  );
}

const KANBAN_TASKS: TaskListItem[] = [
  buildTaskFixture({
    id: "task_k1",
    identifier: "TASK-11",
    status: "pending",
    title: "Refactor event mapper",
    active_run: null,
  }),
  buildTaskFixture({
    id: "task_k2",
    identifier: "TASK-12",
    status: "blocked",
    title: "Await Live bounds review",
    active_run: null,
  }),
  buildTaskFixture({
    id: "task_k7",
    identifier: "TASK-17",
    status: "needs_attention",
    title: "Escalated: unblock-loop breaker tripped",
    active_run: null,
    needs_attention: true,
    needs_attention_reason: "Re-blocked twice with needs_input blocks",
  }),
  buildTaskFixture({
    id: "task_k3",
    identifier: "TASK-13",
    status: "in_progress",
    title: "Streaming buffer leak",
  }),
  buildTaskFixture({
    id: "task_k4",
    identifier: "TASK-14",
    status: "in_progress",
    title: "Bridge health telemetry",
  }),
  buildTaskFixture({
    id: "task_k5",
    identifier: "TASK-15",
    status: "completed",
    title: "Memory GC policy",
    active_run: null,
  }),
  buildTaskFixture({
    id: "task_k6",
    identifier: "TASK-16",
    status: "failed",
    title: "Session timeout retry",
    active_run: {
      id: "run_k6",
      task_id: "task_k6",
      attempt: 3,
      max_attempts: 3,
      recovery_count: 0,
      status: "failed",
      queued_at: "2026-04-17T09:00:00Z",
      error: "session timeout",
    },
  }),
];

function ControlledKanban(
  props: Partial<Parameters<typeof TasksKanbanBoard>[0]> & { tasks?: TaskListItem[] }
) {
  const [selected, setSelected] = useState<string | null>(null);
  const { tasks = KANBAN_TASKS, statusCounts = countTasksByStatus(tasks), ...rest } = props;
  return (
    <TasksKanbanBoard
      columns={groupTasksForKanban(tasks)}
      onSelectTask={setSelected}
      selectedTaskId={selected}
      statusCounts={statusCounts}
      {...rest}
    />
  );
}

export const Populated: Story = {
  args: {},
  render: () => (
    <Frame>
      <ControlledKanban />
    </Frame>
  ),
};

export const Empty: Story = {
  args: {},
  render: () => (
    <Frame>
      <ControlledKanban tasks={[]} />
    </Frame>
  ),
};

export const Loading: Story = {
  args: {},
  render: () => (
    <Frame>
      <ControlledKanban isLoading tasks={[]} />
    </Frame>
  ),
};

export const ErrorState: Story = {
  args: {},
  name: "Error",
  render: () => (
    <Frame>
      <ControlledKanban errorMessage="Failed to load tasks" tasks={[]} />
    </Frame>
  ),
};
