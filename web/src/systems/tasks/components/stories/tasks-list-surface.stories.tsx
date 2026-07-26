import type { Meta, StoryObj } from "@storybook/react-vite";
import { ListChecks } from "lucide-react";
import { useState } from "react";

import { UIProvider, useTopbarSlot } from "@agh/ui";
import { OsWindowFrame } from "@/systems/os/components";

import { TasksListSurface } from "../tasks-list-surface";
import type { TasksListSurfaceProps } from "../tasks-list-surface";
import { TasksListToolbar } from "../tasks-list-toolbar";
import type { TaskFilterOwnerOption } from "../../lib/tasks-list-filters";
import { countTasksByStatus } from "../../lib/task-formatters";
import type { TaskListSortKey, TaskPriority, TaskStatus } from "../../types";
import { buildTaskFixture } from "./fixtures";

const FIXTURE_TASKS = [
  buildTaskFixture({
    id: "task_a",
    status: "in_progress",
    title: "Investigate auth middleware leak",
  }),
  buildTaskFixture({
    id: "task_b",
    status: "ready",
    title: "Wire daemon /healthz to status footer",
    priority: "medium",
  }),
  buildTaskFixture({
    id: "task_c",
    status: "blocked",
    title: "Land RFC-002 protocol receipt schema",
    priority: "urgent",
  }),
  buildTaskFixture({
    id: "task_d",
    status: "pending",
    title: "Define agent execution profile defaults",
    priority: "medium",
  }),
  buildTaskFixture({
    id: "task_e",
    status: "completed",
    title: "Patch SQLite migration registry race on first boot",
    priority: "high",
  }),
  buildTaskFixture({
    id: "task_f",
    status: "failed",
    title: "Backfill task_runs.queue_position for legacy rows",
    priority: "high",
  }),
];

const OWNER_OPTIONS: TaskFilterOwnerOption[] = [
  { ref: "Coder", kind: "agent_session" },
  { ref: "Reviewer", kind: "agent_session" },
  { ref: "pedro@", kind: "human" },
];

function Stateful(props: Partial<TasksListSurfaceProps>) {
  const [statusFilter, setStatusFilter] = useState<TaskStatus | null>(null);
  const [ownerFilter, setOwnerFilter] = useState<TaskFilterOwnerOption | null>(null);
  const [priorityFilter, setPriorityFilter] = useState<TaskPriority | null>(null);
  const [sortBy, setSortBy] = useState<TaskListSortKey>("recent");
  const [searchQuery, setSearchQuery] = useState("");
  const tasks = props.tasks ?? FIXTURE_TASKS;

  return (
    <UIProvider reducedMotion="always">
      <div className="flex h-screen items-center justify-center bg-rail p-10">
        <OsWindowFrame className="h-full max-h-[720px] w-full max-w-[1080px]" title="Tasks">
          <TasksListStoryRoute
            ownerFilter={ownerFilter}
            priorityFilter={priorityFilter}
            props={props}
            searchQuery={searchQuery}
            setOwnerFilter={setOwnerFilter}
            setPriorityFilter={setPriorityFilter}
            setSearchQuery={setSearchQuery}
            setSortBy={setSortBy}
            setStatusFilter={setStatusFilter}
            sortBy={sortBy}
            statusFilter={statusFilter}
            tasks={tasks}
          />
        </OsWindowFrame>
      </div>
    </UIProvider>
  );
}

function TasksListStoryRoute({
  ownerFilter,
  priorityFilter,
  props,
  searchQuery,
  setOwnerFilter,
  setPriorityFilter,
  setSearchQuery,
  setSortBy,
  setStatusFilter,
  sortBy,
  statusFilter,
  tasks,
}: {
  ownerFilter: TaskFilterOwnerOption | null;
  priorityFilter: TaskPriority | null;
  props: Partial<TasksListSurfaceProps>;
  searchQuery: string;
  setOwnerFilter: (value: TaskFilterOwnerOption | null) => void;
  setPriorityFilter: (value: TaskPriority | null) => void;
  setSearchQuery: (value: string) => void;
  setSortBy: (value: TaskListSortKey) => void;
  setStatusFilter: (value: TaskStatus | null) => void;
  sortBy: TaskListSortKey;
  statusFilter: TaskStatus | null;
  tasks: TasksListSurfaceProps["tasks"];
}) {
  useTopbarSlot({
    glyph: <ListChecks />,
    count: tasks.length,
    toolbar: (
      <TasksListToolbar
        onOwnerChange={setOwnerFilter}
        onPriorityChange={setPriorityFilter}
        onSearchQueryChange={setSearchQuery}
        onSortChange={setSortBy}
        onStatusChange={setStatusFilter}
        ownerFilter={ownerFilter}
        ownerOptions={OWNER_OPTIONS}
        priorityFilter={priorityFilter}
        searchQuery={searchQuery}
        sortBy={sortBy}
        statusFilter={statusFilter}
      />
    ),
  });

  return (
    <TasksListSurface
      filterState={
        statusFilter || ownerFilter || priorityFilter || searchQuery ? "active" : "inactive"
      }
      searchQuery={searchQuery}
      statusCounts={props.statusCounts ?? countTasksByStatus(tasks)}
      tasks={tasks}
      {...props}
    />
  );
}

const meta: Meta<typeof TasksListSurface> = {
  title: "systems/tasks/components/TasksListSurface",
  component: TasksListSurface,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component: "Backend-filtered task list with exact status facets and explicit continuation.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof TasksListSurface>;

export const Default: Story = {
  args: {},
  render: () => <Stateful />,
};

export const Loading: Story = {
  args: {},
  render: () => <Stateful isLoading tasks={[]} />,
};

export const Empty: Story = {
  args: {},
  render: () => <Stateful tasks={[]} />,
};

export const ErrorState: Story = {
  args: {},
  render: () => <Stateful tasks={[]} errorMessage="Daemon unreachable on /api/tasks." />,
};

export const SingleGroup: Story = {
  args: {},
  render: () => <Stateful tasks={FIXTURE_TASKS.filter(task => task.status === "in_progress")} />,
};
