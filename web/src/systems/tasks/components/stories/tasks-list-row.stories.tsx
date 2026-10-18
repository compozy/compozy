import type { Meta, StoryObj } from "@storybook/react-vite";

import { Table } from "@compozy/ui";

import { PanelSurface } from "@/storybook/story-layout";
import type { TaskListItem } from "../../types";
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

/** The Tasks table at a half-pane width: Task · Status · Owner (ID from 512px). */
function Frame({ children, wide = false }: { children: React.ReactNode; wide?: boolean }) {
  return (
    <PanelSurface className={wide ? "@container max-w-240 p-0" : "@container max-w-170 p-0"}>
      <Table className="table-fixed" overflowX="hidden">
        {children}
      </Table>
    </PanelSurface>
  );
}

function Row({ task }: { task: TaskListItem }) {
  return (
    <tbody>
      <TaskCard task={task} />
    </tbody>
  );
}

export const Pending: Story = {
  render: () => (
    <Frame>
      <Row
        task={buildTaskFixture({ status: "pending", title: "Pending task", active_run: null })}
      />
    </Frame>
  ),
};

export const Running: Story = {
  render: () => (
    <Frame>
      <Row task={buildTaskFixture({ status: "in_progress", title: "Running task" })} />
    </Frame>
  ),
};

export const Done: Story = {
  render: () => (
    <Frame>
      <Row task={buildTaskFixture({ status: "completed", title: "Done task", active_run: null })} />
    </Frame>
  ),
};

export const Failed: Story = {
  render: () => (
    <Frame>
      <Row task={buildTaskFixture({ status: "failed", title: "Failed task", active_run: null })} />
    </Frame>
  ),
};

export const Blocked: Story = {
  render: () => (
    <Frame>
      <Row
        task={buildTaskFixture({ status: "blocked", title: "Blocked task", active_run: null })}
      />
    </Frame>
  ),
};

/** Status groups as table bodies, each led by its sunken glyph header (production list layout). */
export const ListGroups: Story = {
  render: () => (
    <Frame wide>
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
    </Frame>
  ),
};
