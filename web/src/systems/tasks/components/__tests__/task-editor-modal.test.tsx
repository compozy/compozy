import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import {
  TaskEditorModal,
  type TaskEditorModalMode,
  type TaskEditorModalStatus,
} from "../task-editor-modal";
import {
  createTaskEditorDraft,
  taskEditorDraftFromTask,
  type TaskEditorDraft,
} from "../../lib/task-editor";
import type { TaskTemplateId } from "../../lib/task-templates";
import { buildTaskExecutionProfileFixture } from "../../mocks/fixtures";
import type { TaskRecord } from "../../types";

interface RenderModalOptions {
  mode?: TaskEditorModalMode;
  templateId?: TaskTemplateId;
  draft?: TaskEditorDraft;
  canSubmit?: boolean;
  isSubmitting?: boolean;
  status?: TaskEditorModalStatus;
}

const editTask = {
  id: "task_42",
  identifier: "TASK-42",
  title: "Summarize review feedback",
  status: "in_progress",
  scope: "workspace",
  origin: { kind: "cli", ref: "op" },
  workspace_id: "ws_alpha",
  created_at: "2026-04-11T09:00:00Z",
  updated_at: "2026-04-11T09:30:00Z",
  created_by: { kind: "human", ref: "pedro" },
} as unknown as TaskRecord;

const workspaces = [
  { id: "ws_alpha", name: "launch-hq" },
  { id: "ws_beta", name: "risk-ops" },
];

/**
 * Controlled harness — holds the draft + active template in local state and
 * syncs every `onDraftChange` (value or updater function) and `onTemplateChange`
 * back into state so multi-step interactions (e.g. open Execution → flip Save as
 * draft → read the submit label) persist across re-renders.
 */
function renderModal({
  mode = "new",
  templateId = "one_shot",
  draft = createTaskEditorDraft(templateId, "ws_alpha"),
  canSubmit,
  isSubmitting = false,
  status = "ready",
}: RenderModalOptions = {}) {
  const onOpenChange = vi.fn();
  const onTemplateChange = vi.fn();
  const onSubmit = vi.fn().mockResolvedValue(undefined);
  const onDraftChange = vi.fn();
  const isNewMode = mode === "new";

  function Harness() {
    const [currentTemplate, setCurrentTemplate] = useState<TaskTemplateId>(templateId);
    const [currentDraft, setCurrentDraft] = useState<TaskEditorDraft>(draft);

    return (
      <TaskEditorModal
        canSubmit={canSubmit ?? currentDraft.title.trim().length > 0}
        draft={currentDraft}
        isSubmitting={isSubmitting}
        mode={mode}
        onDraftChange={next => {
          setCurrentDraft(prev => {
            const resolved = typeof next === "function" ? next(prev) : next;
            onDraftChange(resolved);
            return resolved;
          });
        }}
        onOpenChange={onOpenChange}
        onSubmit={onSubmit}
        onTemplateChange={
          isNewMode
            ? next => {
                onTemplateChange(next);
                setCurrentTemplate(next);
                setCurrentDraft(createTaskEditorDraft(next, "ws_alpha"));
              }
            : undefined
        }
        open
        status={status}
        templateId={isNewMode ? currentTemplate : undefined}
        workspaces={workspaces}
      />
    );
  }

  render(<Harness />);

  return { onOpenChange, onTemplateChange, onSubmit, onDraftChange };
}

describe("TaskEditorModal", () => {
  it("Should render the create header and Simple template grid by default", () => {
    renderModal();

    const modal = screen.getByTestId("task-editor-modal");
    expect(modal).toBeInTheDocument();
    // The shared entity header owns the eyebrow, title, and description.
    const header = modal.querySelector('[data-slot="entity-dialog-header"]');
    expect(header).not.toBeNull();
    expect(within(header as HTMLElement).getByText("Autonomy · Task")).toBeInTheDocument();
    expect(within(header as HTMLElement).getByText("Create task")).toBeInTheDocument();
    expect(
      within(header as HTMLElement).getByText(/A task is a durable contract/)
    ).toBeInTheDocument();
    // The description no longer duplicates as a paragraph in the scrolling body.
    expect(
      within(screen.getByTestId("task-editor-modal-body")).queryByText(
        /A task is a durable contract/
      )
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("task-mode-simple")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("task-template-one_shot")).toBeInTheDocument();
    expect(screen.getByTestId("task-template-human_in_loop")).toBeInTheDocument();
    expect(screen.getByTestId("task-template-epic")).toBeInTheDocument();
    // Advanced-only sections stay hidden in Simple mode.
    expect(screen.queryByTestId("task-parent-input")).not.toBeInTheDocument();
  });

  it("Should open an advanced-only template in Advanced mode", () => {
    renderModal({ templateId: "recurring" });

    expect(screen.getByTestId("task-mode-advanced")).toHaveAttribute("aria-pressed", "true");
    expect(screen.getByTestId("task-template-recurring")).toHaveAttribute("aria-checked", "true");
    expect(screen.getByTestId("task-parent-input")).toBeInTheDocument();
  });

  it("Should expose exactly one close route", () => {
    renderModal();

    // The header owns dismissal; DialogContent's stock close stays disabled so
    // the two never render together.
    const closes = screen.getAllByRole("button", { name: /close/i });
    expect(closes).toHaveLength(1);
    expect(closes[0]).toHaveAttribute("data-slot", "entity-dialog-header-close");
    expect(
      screen.getByTestId("task-editor-modal").querySelector('[data-slot="dialog-close"]')
    ).toBeNull();
  });

  it("Should dismiss through the header close control", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderModal();

    await user.click(screen.getByRole("button", { name: /close/i }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should switch to Advanced and reveal the advanced numbered sections", () => {
    renderModal();

    fireEvent.click(screen.getByTestId("task-mode-advanced"));

    expect(screen.getByTestId("task-mode-advanced")).toHaveAttribute("aria-pressed", "true");
    // Placement
    expect(screen.getByTestId("task-parent-input")).toBeInTheDocument();
    // Queue & ownership
    expect(screen.getByTestId("task-owner-kind")).toBeInTheDocument();
    expect(screen.getByTestId("task-attempts-options")).toBeInTheDocument();
    // Ingress & identity
    expect(screen.getByTestId("task-identifier-input")).toBeInTheDocument();
    // Execution collapsible
    expect(screen.getByTestId("task-execution-toggle")).toBeInTheDocument();
    // Advanced is a disclosure tier, not a replacement: required fields stay put.
    expect(screen.getByTestId("task-title-input")).toBeInTheDocument();
  });

  it("Should snap an advanced-only template back to a Simple-valid default on leaving Advanced", () => {
    const { onTemplateChange } = renderModal({ templateId: "recurring" });

    expect(onTemplateChange).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("task-mode-simple"));
    expect(onTemplateChange).toHaveBeenCalledWith("one_shot");
  });

  it("Should keep a Simple-valid template when leaving Advanced", () => {
    const { onTemplateChange } = renderModal({ templateId: "human_in_loop" });

    fireEvent.click(screen.getByTestId("task-mode-advanced"));
    fireEvent.click(screen.getByTestId("task-mode-simple"));

    expect(onTemplateChange).not.toHaveBeenCalled();
  });

  it("Should update the title and propagate it through onDraftChange", () => {
    const { onDraftChange } = renderModal();

    fireEvent.change(screen.getByTestId("task-title-input"), {
      target: { value: "Generate API client for payments-v3" },
    });

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ title: "Generate API client for payments-v3" })
    );
    expect(screen.getByTestId("task-title-input")).toHaveValue(
      "Generate API client for payments-v3"
    );
  });

  it("Should change the priority through the priority pill group", () => {
    const { onDraftChange } = renderModal();

    fireEvent.click(screen.getByTestId("task-priority-high"));

    expect(onDraftChange).toHaveBeenLastCalledWith(expect.objectContaining({ priority: "high" }));
    expect(screen.getByTestId("task-priority-high")).toHaveAttribute("aria-pressed", "true");
  });

  it("Should select a workspace through the shared scope selector", async () => {
    const user = userEvent.setup();
    const { onDraftChange } = renderModal();

    await user.click(screen.getByTestId("task-workspace-select"));
    await user.click(screen.getByTestId("task-workspace-item-ws_beta"));

    expect(onDraftChange).toHaveBeenLastCalledWith(
      expect.objectContaining({ scope: "workspace", workspaceId: "ws_beta" })
    );
    expect(screen.getByTestId("workspace-switcher-name")).toHaveTextContent("risk-ops");
  });

  it("Should select a template card and emit onTemplateChange", () => {
    const { onTemplateChange } = renderModal();

    fireEvent.click(screen.getByTestId("task-template-human_in_loop"));

    expect(onTemplateChange).toHaveBeenCalledWith("human_in_loop");
    expect(screen.getByTestId("task-template-human_in_loop")).toHaveAttribute(
      "aria-checked",
      "true"
    );
  });

  it("Should toggle Save as draft and flip the submit label to Save draft", () => {
    const { onDraftChange } = renderModal();

    fireEvent.click(screen.getByTestId("task-mode-advanced"));
    // Execution panel is collapsed until expanded.
    expect(screen.queryByTestId("task-save-draft-toggle")).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId("task-execution-toggle"));
    fireEvent.click(screen.getByTestId("task-save-draft-toggle"));

    expect(onDraftChange).toHaveBeenLastCalledWith(expect.objectContaining({ saveAsDraft: true }));
    expect(screen.getByTestId("task-editor-modal-submit")).toHaveTextContent("Save draft");
  });

  it("Should call onSubmit when the form is submitted", () => {
    const { onSubmit } = renderModal({
      draft: { ...createTaskEditorDraft("one_shot", "ws_alpha"), title: "Ship it" },
    });

    fireEvent.submit(screen.getByTestId("task-editor-modal-form"));

    expect(onSubmit).toHaveBeenCalledOnce();
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({ title: "Ship it" }), false);
  });

  it("Should render edit mode without the toolbar, templates, or network channel input", () => {
    const draft: TaskEditorDraft = {
      ...createTaskEditorDraft("blank", "ws_alpha"),
      title: "Summarize review feedback",
      maxAttempts: 3,
    };

    renderModal({ mode: "edit", draft });

    const header = screen
      .getByTestId("task-editor-modal")
      .querySelector('[data-slot="entity-dialog-header"]');
    expect(within(header as HTMLElement).getByText("Edit task")).toBeInTheDocument();
    expect(screen.queryByTestId("task-mode-simple")).not.toBeInTheDocument();
    expect(screen.queryByTestId("task-mode-advanced")).not.toBeInTheDocument();
    expect(screen.queryByTestId("task-template-one_shot")).not.toBeInTheDocument();
    expect(screen.getByTestId("task-title-input")).toHaveValue("Summarize review feedback");
    expect(screen.queryByTestId("task-network-input")).not.toBeInTheDocument();
    expect(screen.getByTestId("task-editor-modal-submit")).toHaveTextContent("Save changes");
  });

  it("Should hold the edit host on a loading state instead of a half-bound form", () => {
    renderModal({ mode: "edit", status: "loading" });

    expect(screen.getByTestId("task-editor-modal-loading")).toBeInTheDocument();
    expect(screen.queryByTestId("task-editor-modal-form")).not.toBeInTheDocument();
    const header = screen
      .getByTestId("task-editor-modal")
      .querySelector('[data-slot="entity-dialog-header"]');
    expect(within(header as HTMLElement).getByText("Edit task")).toBeInTheDocument();
  });

  it("Should report an unreadable task instead of rendering an empty editor", async () => {
    const user = userEvent.setup();
    const { onOpenChange } = renderModal({ mode: "edit", status: "unavailable" });

    expect(screen.getByTestId("task-editor-modal-unavailable")).toBeInTheDocument();
    expect(screen.queryByTestId("task-editor-modal-form")).not.toBeInTheDocument();
    // Dismissal stays reachable so the host can navigate back to the task.
    await user.click(screen.getByRole("button", { name: /close/i }));
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("Should submit persisted Live participation inherited from a Loop run", () => {
    const loopTask = {
      ...editTask,
      execution_mode: "loop_run",
    } as TaskRecord;
    const draft = taskEditorDraftFromTask(
      loopTask,
      buildTaskExecutionProfileFixture({
        task_id: loopTask.id,
        network_participation: {
          mode: "live",
          channel_strategy: "loop_run",
        },
      })
    );

    const { onSubmit } = renderModal({ mode: "edit", draft });

    expect(screen.getByTestId("task-editor-participation-mode")).toHaveValue("live");
    expect(screen.getByTestId("task-editor-participation-strategy")).toHaveValue("loop_run");
    expect(screen.queryByTestId("task-editor-participation-channel")).not.toBeInTheDocument();
    expect(screen.getByTestId("task-editor-modal-submit")).toBeEnabled();

    fireEvent.submit(screen.getByTestId("task-editor-modal-form"));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        networkParticipationMode: "live",
        networkChannelId: "",
        networkChannelStrategy: "loop_run",
      }),
      false
    );
  });
});
