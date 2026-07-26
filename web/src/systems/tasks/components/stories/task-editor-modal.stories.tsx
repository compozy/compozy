import { useState } from "react";
import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent, within } from "storybook/test";

import { storyWorkspaceIds, storyWorkspaceNames } from "@/storybook/fintech-scenario";
import { CenteredSurface } from "@/storybook/story-layout";

import {
  TaskEditorModal,
  type TaskEditorModalMode,
  type TaskEditorModalStatus,
} from "../task-editor-modal";
import {
  applyTaskTemplateToEditorDraft,
  createTaskEditorDraft,
  type TaskEditorDraft,
} from "../../lib/task-editor";
import { DEFAULT_TASK_TEMPLATE_ID, type TaskTemplateId } from "../../lib/task-templates";

const meta: Meta<typeof TaskEditorModal> = {
  title: "systems/tasks/components/TaskEditorModal",
  component: TaskEditorModal,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

const storyWorkspaces = [
  { id: storyWorkspaceIds.hq, name: storyWorkspaceNames.hq },
  { id: storyWorkspaceIds.risk, name: storyWorkspaceNames.risk },
  { id: storyWorkspaceIds.growth, name: storyWorkspaceNames.growth },
];

interface TaskEditorModalHarnessProps {
  mode?: TaskEditorModalMode;
  templateId?: TaskTemplateId;
  initialDraft?: TaskEditorDraft;
  isSubmitting?: boolean;
  status?: TaskEditorModalStatus;
}

/**
 * Controlled Storybook harness — holds the editor draft and the active template
 * id in local state, mirroring the live `useTasksPage` orchestration. The modal
 * stays open and renders on the centered story surface; submit / open-change are
 * no-ops so the surface is stable for visual capture.
 */
function TaskEditorModalHarness({
  mode = "new",
  templateId = DEFAULT_TASK_TEMPLATE_ID,
  initialDraft,
  isSubmitting = false,
  status = "ready",
}: TaskEditorModalHarnessProps) {
  const [activeTemplate, setActiveTemplate] = useState<TaskTemplateId>(templateId);
  const [draft, setDraft] = useState<TaskEditorDraft>(
    () => initialDraft ?? createTaskEditorDraft(templateId, storyWorkspaceIds.hq)
  );

  const isNewMode = mode === "new";
  const canSubmit = draft.title.trim().length > 0;

  return (
    <CenteredSurface className="items-start justify-center p-0">
      <TaskEditorModal
        canSubmit={canSubmit}
        draft={draft}
        isSubmitting={isSubmitting}
        mode={mode}
        onDraftChange={setDraft}
        onOpenChange={() => undefined}
        onSubmit={() => Promise.resolve()}
        onTemplateChange={
          isNewMode
            ? next => {
                setActiveTemplate(next);
                setDraft(current => applyTaskTemplateToEditorDraft(current, next));
              }
            : undefined
        }
        open
        status={status}
        templateId={isNewMode ? activeTemplate : undefined}
        workspaces={storyWorkspaces}
      />
    </CenteredSurface>
  );
}

function buildEditDraft(): TaskEditorDraft {
  return {
    ...createTaskEditorDraft("blank", storyWorkspaceIds.hq),
    title: "Summarize launch-room escalations",
    description: "Compress the escalation thread into five bullets the launch room can act on.",
    priority: "high",
    maxAttempts: 3,
  };
}

function buildAuthoredCreateDraft(): TaskEditorDraft {
  return {
    ...createTaskEditorDraft("one_shot", storyWorkspaceIds.hq),
    title: "Preserve release contract",
    description: "Keep this authored description while changing the task template.",
  };
}

function buildExactOwnerEditDraft(): TaskEditorDraft {
  return {
    ...buildEditDraft(),
    ownerKind: "agent_session",
    ownerRef: "sess-exact-launch-owner",
  };
}

/**
 * Default create surface — Simple mode with the three approachable template
 * cards. Click the Advanced pill (`task-mode-advanced`) to reveal the numbered
 * Placement / Queue / Ingress sections and the Execution collapsible.
 */
export const NewSimple: Story = {
  args: {},
  render: () => <TaskEditorModalHarness />,
};

/**
 * Advanced create surface — drives the internal Simple/Advanced toggle through a
 * play function so the numbered advanced sections render on load.
 */
export const NewAdvanced: Story = {
  args: {},
  render: () => <TaskEditorModalHarness />,
  play: async ({ canvasElement }) => {
    const advancedPill = canvasElement.querySelector<HTMLButtonElement>(
      "[data-testid='task-mode-advanced']"
    );
    advancedPill?.click();
  },
};

/**
 * Authored contract retained after switching from Run once to Needs approval.
 */
export const DraftRetainedAfterTemplateSwitch: Story = {
  args: {},
  render: () => <TaskEditorModalHarness initialDraft={buildAuthoredCreateDraft()} />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    await userEvent.click(await canvas.findByTestId("task-template-human_in_loop"));
    await expect(canvas.getByTestId("task-title-input")).toHaveValue("Preserve release contract");
    await expect(canvas.getByTestId("task-description-input")).toHaveValue(
      "Keep this authored description while changing the task template."
    );
  },
};

/**
 * Recurring template — saves as a draft so Automation can attach the schedule
 * later; the submit label reads `Save draft`.
 */
export const RecurringDraft: Story = {
  args: {},
  render: () => <TaskEditorModalHarness templateId="recurring" />,
};

/** Edit state after changing an exact-session owner to Unassigned. */
export const EditModeUnassigned: Story = {
  args: {},
  render: () => <TaskEditorModalHarness initialDraft={buildExactOwnerEditDraft()} mode="edit" />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement.ownerDocument.body);
    const ownerKind = await canvas.findByTestId("task-owner-kind");
    await userEvent.selectOptions(ownerKind, "");
    ownerKind.scrollIntoView({ block: "center" });
    await expect(ownerKind).toHaveValue("");
    await expect(canvas.getByTestId("task-owner-ref")).toBeDisabled();
    await expect(canvas.getByTestId("task-owner-ref")).toHaveValue("");
  },
};

/** Validation disabled — empty title keeps the submit action disabled. */
export const ValidationDisabled: Story = {
  args: {},
  render: () => (
    <TaskEditorModalHarness
      initialDraft={createTaskEditorDraft(DEFAULT_TASK_TEMPLATE_ID, storyWorkspaceIds.hq)}
    />
  ),
};

/** Edit host while the bound task is still resolving. */
export const EditLoading: Story = {
  args: {},
  render: () => <TaskEditorModalHarness mode="edit" status="loading" />,
};

/** Edit host when the bound task cannot be read — the form never renders half-bound. */
export const EditUnavailable: Story = {
  args: {},
  render: () => <TaskEditorModalHarness mode="edit" status="unavailable" />,
};
