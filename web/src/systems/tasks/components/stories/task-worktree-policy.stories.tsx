import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn, userEvent, within } from "storybook/test";

import { StorySurface } from "@/storybook/story-layout";
import {
  worktreeBehindFixture,
  worktreeMergedFixture,
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import type { TaskProfileEditor } from "../../hooks/use-profile-editor";
import { taskExecutionProfileFixture } from "../../mocks";
import type { TaskExecutionProfileWorktree } from "../../types";
import { TaskSetupSheet } from "../task-setup-sheet";
import { TaskWorktreePolicyFields } from "../task-worktree-policy-fields";
import { TaskWorktreePolicyReadRow } from "../task-worktree-policy-read-row";

const WORKTREES = [
  worktreeReadyDirtyRunningFixture,
  worktreeBehindFixture,
  worktreeMergedFixture,
  worktreeMissingFixture,
];

const WORKSPACE_PATH = "/Users/ada/dev/compozy";

function Harness({
  initial,
  locked = false,
}: {
  initial: TaskExecutionProfileWorktree;
  locked?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <StorySurface className="w-[520px]">
      <TaskWorktreePolicyFields
        locked={locked}
        onChange={setValue}
        value={value}
        workspacePath={WORKSPACE_PATH}
        worktrees={WORKTREES}
      />
    </StorySurface>
  );
}

const meta: Meta<typeof TaskWorktreePolicyFields> = {
  title: "systems/tasks/components/TaskWorktreePolicy",
  component: TaskWorktreePolicyFields,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Where a task's runs execute. The mode vocabulary is the locked one shared with the session, loop, and node controls, and every change writes through the dedicated policy patch rather than a profile replace.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-18 — Inherit: the task declares nothing and follows the workspace. */
export const ModeInherit: Story = {
  args: { value: { mode: "inherit" }, worktrees: WORKTREES, onChange: fn() },
  render: () => <Harness initial={{ mode: "inherit" }} />,
};

/** VC-19 — Workspace root, with the path the runs will actually use. */
export const ModeWorkspaceRoot: Story = {
  args: { ...ModeInherit.args, value: { mode: "none" } },
  render: () => <Harness initial={{ mode: "none" }} />,
};

/** VC-20 — Named worktree with the picker open: only ready rows are offers. */
export const ModeNamedWorktreePickerOpen: Story = {
  args: { ...ModeInherit.args, value: { mode: "ref", worktree_ref: worktreeBehindFixture.id } },
  render: () => <Harness initial={{ mode: "ref", worktree_ref: worktreeBehindFixture.id }} />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("tasks-setup-worktree-ref"));
  },
};

/** VC-21 — Per-run, stating what each run gets and what happens to the work after. */
export const ModePerRun: Story = {
  args: { ...ModeInherit.args, value: { mode: "per_run" } },
  render: () => <Harness initial={{ mode: "per_run" }} />,
};

/** VC-22 — the referenced worktree is gone: flagged, never presented as settled. */
export const InvalidRemovedRef: Story = {
  args: { ...ModeInherit.args, value: { mode: "ref", worktree_ref: worktreeMissingFixture.id } },
  render: () => <Harness initial={{ mode: "ref", worktree_ref: worktreeMissingFixture.id }} />,
};

/** VC-23 — a run is active, so the policy is read-only until it ends. */
export const SheetLockedDuringActiveRun: Story = {
  args: ModeInherit.args,
  render: () => {
    const editor: TaskProfileEditor = {
      error: null,
      open: true,
      setOpen: fn(),
      setValue: fn(),
      submit: fn(),
      value: {
        ...taskExecutionProfileFixture,
        task_id: taskExecutionProfileFixture.task_id,
        worktree: { mode: "ref", worktree_ref: worktreeBehindFixture.id },
      },
    };
    return (
      <TaskSetupSheet
        activeRunId="run_fanout_2"
        clearOpen={false}
        editor={editor}
        hasActiveRun
        onClearOpenChange={fn()}
        onClearProfile={fn()}
        onOpenChange={fn()}
        open
        profile={{
          ...taskExecutionProfileFixture,
          worktree: { mode: "ref", worktree_ref: worktreeBehindFixture.id },
        }}
        runtime={{
          catalogLoading: false,
          catalogRefreshing: false,
          models: [],
          onRefreshCatalog: fn(),
          providers: [],
          providersLoading: false,
        }}
        worktreePolicy={{
          locked: true,
          onChange: fn(),
          value: { mode: "ref", worktree_ref: worktreeBehindFixture.id },
          workspacePath: WORKSPACE_PATH,
          worktrees: WORKTREES,
        }}
      />
    );
  },
};

/** VC-24 — the profile read view: the record label, never the branch or basename. */
export const ReadRowWithPolicy: Story = {
  args: ModeInherit.args,
  render: () => (
    <StorySurface className="w-[520px]">
      <TaskWorktreePolicyReadRow
        value={{ mode: "ref", worktree_ref: worktreeBehindFixture.id }}
        worktrees={WORKTREES}
      />
    </StorySurface>
  ),
};
