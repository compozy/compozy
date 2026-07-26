import type { Meta, StoryObj } from "@storybook/react-vite";

import { PanelSurface } from "@/storybook/story-layout";
import { TaskCard } from "../task-card";
import { TaskGroup } from "../task-group";
import { TasksListRow } from "../tasks-list-row";
import { buildTaskFixture, TASK_FIXTURES } from "./fixtures";

const meta: Meta<typeof TasksListRow> = {
  title: "systems/tasks/components/TasksListRow",
  component: TasksListRow,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

function Frame({ children }: { children: React.ReactNode }) {
  return <PanelSurface className="max-w-[340px] p-0">{children}</PanelSurface>;
}

export const Pending: Story = {
  render: () => (
    <Frame>
      <TasksListRow
        task={buildTaskFixture({ status: "pending", title: "Pending task", active_run: null })}
      />
    </Frame>
  ),
};

export const Running: Story = {
  render: () => (
    <Frame>
      <TasksListRow task={buildTaskFixture({ status: "in_progress", title: "Running task" })} />
    </Frame>
  ),
};

export const Done: Story = {
  render: () => (
    <Frame>
      <TasksListRow
        task={buildTaskFixture({ status: "completed", title: "Done task", active_run: null })}
      />
    </Frame>
  ),
};

export const Failed: Story = {
  render: () => (
    <Frame>
      <TasksListRow
        task={buildTaskFixture({ status: "failed", title: "Failed task", active_run: null })}
      />
    </Frame>
  ),
};

export const Blocked: Story = {
  render: () => (
    <Frame>
      <TasksListRow
        task={buildTaskFixture({ status: "blocked", title: "Blocked task", active_run: null })}
      />
    </Frame>
  ),
};

/** List groups with header dots and flush-left task cards (production list layout). */
export const ListGroups: Story = {
  render: () => (
    <PanelSurface className="max-w-[520px] p-0">
      <TaskGroup count={1} id="active" label="Active">
        <TaskCard task={TASK_FIXTURES[0]!} />
      </TaskGroup>
      <TaskGroup count={1} id="blocked" label="Blocked">
        <TaskCard task={TASK_FIXTURES[5]!} />
      </TaskGroup>
      <TaskGroup count={1} id="queued" label="Queued">
        <TaskCard task={TASK_FIXTURES[2]!} />
      </TaskGroup>
      <TaskGroup count={1} id="done" label="Done">
        <TaskCard
          task={buildTaskFixture({ status: "completed", title: "Done task", active_run: null })}
        />
      </TaskGroup>
      <TaskGroup count={1} id="failed" label="Failed">
        <TaskCard task={TASK_FIXTURES[3]!} />
      </TaskGroup>
    </PanelSurface>
  ),
};
