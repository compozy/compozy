"use client";

import { Check } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  EntityDialogBody,
  EntityDialogFooter,
  EntityModeToolbar,
  FormSection,
  type EntityMode,
} from "@agh/ui";

import {
  NetworkParticipationFields,
  isNetworkParticipationDraftValid,
  networkParticipationDraftFromValues,
} from "@/systems/network";
import { ScopeSelector, type WorkspaceCommandSelectOption } from "@/systems/workspace";

import type { TaskEditorDraft } from "../lib/task-editor";
import { SIMPLE_TASK_TEMPLATE_IDS, type TaskTemplateId } from "../lib/task-templates";
import { ContractSection } from "./task-form/contract-section";
import { ExecutionCollapsible } from "./task-form/execution-collapsible";
import { IngressIdentitySection } from "./task-form/ingress-identity-section";
import { PlacementSection } from "./task-form/placement-section";
import { PrioritySection } from "./task-form/priority-section";
import { QueueOwnershipSection } from "./task-form/queue-ownership-section";
import { TemplateCards } from "./task-form/template-cards";
import { useTasksCreateModalForm } from "./use-tasks-create-modal-form";

export type TaskEditorSurfaceMode = "new" | "edit";

export interface TaskEditorSurfaceProps {
  mode: TaskEditorSurfaceMode;
  /**
   * Dialog chrome supplied by the modal host. `EntityDialogHeader` renders
   * `DialogTitle`, so it can only be mounted inside a `Dialog`; the host owns it
   * and hands it down as a slot.
   */
  header: ReactNode;
  draft: TaskEditorDraft;
  onDraftChange: (next: TaskEditorDraft | ((current: TaskEditorDraft) => TaskEditorDraft)) => void;
  onSubmit: (draft: TaskEditorDraft, asDraft: boolean) => Promise<unknown> | void;
  onCancel: () => void;
  canSubmit?: boolean;
  isSubmitting?: boolean;
  workspaces?: ReadonlyArray<WorkspaceCommandSelectOption>;
  templateId?: TaskTemplateId;
  onTemplateChange?: (templateId: TaskTemplateId) => void;
}

export const TASK_DESCRIPTION =
  "A task is a durable contract — a unit of work that gets claimed and run by an owner. Runs descend from it and respect its dependencies.";

function resolveSubmitLabel(mode: TaskEditorSurfaceMode, saveAsDraft: boolean): string {
  if (mode === "edit") return "Save changes";
  return saveAsDraft ? "Save draft" : "Enqueue task";
}

export function TaskEditorSurface({
  mode,
  header,
  draft,
  onDraftChange,
  onSubmit,
  onCancel,
  canSubmit = true,
  isSubmitting = false,
  workspaces,
  templateId,
  onTemplateChange,
}: TaskEditorSurfaceProps) {
  const form = useTasksCreateModalForm({ draft, onDraftChange, onSubmit });
  const isNewMode = mode === "new";
  const [formMode, setFormMode] = useState<EntityMode>(() =>
    isNewMode && templateId && !SIMPLE_TASK_TEMPLATE_IDS.includes(templateId)
      ? "advanced"
      : "simple"
  );

  const setField = (field: keyof TaskEditorDraft) => (value: string) =>
    onDraftChange(current => ({ ...current, [field]: value }));

  const handleModeChange = (nextMode: EntityMode) => {
    setFormMode(nextMode);
    if (
      nextMode === "simple" &&
      templateId &&
      !SIMPLE_TASK_TEMPLATE_IDS.includes(templateId) &&
      onTemplateChange
    ) {
      onTemplateChange("one_shot");
    }
  };

  const advanced = isNewMode && formMode === "advanced";
  const submitLabel = resolveSubmitLabel(mode, draft.saveAsDraft);
  const networkParticipation = networkParticipationDraftFromValues(
    draft.networkParticipationMode,
    draft.networkChannelId,
    draft.networkChannelStrategy
  );
  const participationValid = isNetworkParticipationDraftValid(networkParticipation, [
    "named",
    "run",
    "loop_run",
  ]);
  const submitAllowed = canSubmit && participationValid;

  return (
    <section
      className="flex min-h-0 flex-1 flex-col bg-canvas text-fg"
      data-mode={mode}
      data-testid="task-editor-surface"
    >
      {header}
      <form
        className="flex min-h-0 flex-1 flex-col"
        data-testid="task-editor-modal-form"
        onSubmit={event => {
          if (!participationValid) {
            event.preventDefault();
            return;
          }
          form.submitForm(event);
        }}
      >
        {isNewMode ? (
          <EntityModeToolbar
            mode={formMode}
            onModeChange={handleModeChange}
            testIdPrefix="task"
            trailing={
              <ScopeSelector
                ariaLabel="Task scope"
                onScopeChange={form.updateScope}
                onWorkspaceChange={form.updateWorkspace}
                scope={draft.scope}
                testIdPrefix="task"
                workspaceId={draft.workspaceId}
                workspaces={workspaces}
              />
            }
            trailingLabel="Scope"
          />
        ) : null}

        <EntityDialogBody data-testid="task-editor-modal-body">
          <FormSection
            help="The title and description a person or agent reads when the task is claimed."
            title="The contract"
          >
            <ContractSection
              description={draft.description}
              onDescription={setField("description")}
              onTitle={setField("title")}
              title={draft.title}
            />
          </FormSection>

          {isNewMode && templateId && onTemplateChange ? (
            <FormSection
              help={
                advanced
                  ? "Presets the contract fields below — tweak any of them."
                  : "Pick the closest fit — you can change details after."
              }
              title={advanced ? "Template" : "How should it run?"}
            >
              <TemplateCards mode={formMode} onSelect={onTemplateChange} templateId={templateId} />
            </FormSection>
          ) : null}

          <FormSection help="Higher priority gets claimed sooner." title="Priority">
            <PrioritySection onPriority={form.updatePriority} priority={draft.priority} />
          </FormSection>

          <FormSection
            description="Local by default. Live requires an explicit channel strategy."
            title="Network participation"
          >
            <NetworkParticipationFields
              allowedStrategies={["named", "run", "loop_run"]}
              onChange={next =>
                onDraftChange(current => ({
                  ...current,
                  networkParticipationMode: next.mode,
                  networkChannelId: next.channelId,
                  networkChannelStrategy: next.channelStrategy,
                }))
              }
              testIdPrefix="task-editor-participation"
              value={networkParticipation}
            />
          </FormSection>

          {advanced ? (
            <>
              <FormSection help="Where it sits in the task hierarchy." title="Placement">
                <PlacementSection
                  onParentTaskId={setField("parentTaskId")}
                  parentTaskId={draft.parentTaskId}
                />
              </FormSection>
              <FormSection
                help="Who runs it, and how retries behave."
                title="Queue &amp; ownership"
              >
                <QueueOwnershipSection
                  approvalPolicy={draft.approvalPolicy}
                  maxAttempts={draft.maxAttempts}
                  onApprovalPolicy={form.updateApprovalPolicy}
                  onMaxAttempts={form.updateMaxAttempts}
                  onOwnerKind={form.updateOwnerKind}
                  onOwnerRef={setField("ownerRef")}
                  ownerKind={draft.ownerKind}
                  ownerRef={draft.ownerRef}
                />
              </FormSection>
              <FormSection
                help="A stable identifier override. Leave it empty and the daemon assigns one."
                title="Identity"
              >
                <IngressIdentitySection
                  identifier={draft.identifier}
                  onIdentifier={setField("identifier")}
                />
              </FormSection>
              <ExecutionCollapsible
                autoEnqueueOnReady={draft.autoEnqueueOnReady}
                onAutoEnqueue={form.updateAutoEnqueue}
                onSaveAsDraft={form.updateSaveAsDraft}
                saveAsDraft={draft.saveAsDraft}
              />
            </>
          ) : null}

          {!isNewMode ? (
            <FormSection help="Who runs it, and how retries behave." title="Queue &amp; ownership">
              <QueueOwnershipSection
                approvalPolicy={draft.approvalPolicy}
                maxAttempts={draft.maxAttempts}
                onApprovalPolicy={form.updateApprovalPolicy}
                onMaxAttempts={form.updateMaxAttempts}
                onOwnerKind={form.updateOwnerKind}
                onOwnerRef={setField("ownerRef")}
                ownerKind={draft.ownerKind}
                ownerRef={draft.ownerRef}
              />
            </FormSection>
          ) : null}
        </EntityDialogBody>

        <EntityDialogFooter
          cancelTestId="task-editor-modal-cancel"
          hint={
            draft.saveAsDraft ? (
              <>
                Saved as a <b className="font-medium text-muted">draft</b>; no run is queued until
                you enqueue it.
              </>
            ) : (
              <>The contract is durable; runs descend from this task and respect dependencies.</>
            )
          }
          hintTestId="task-editor-modal-hint"
          isSaving={isSubmitting}
          onCancel={onCancel}
          primaryDisabled={!submitAllowed}
          primaryIcon={mode === "new" ? Check : undefined}
          primaryLabel={submitLabel}
          primaryTestId="task-editor-modal-submit"
          primaryType="submit"
        />
      </form>
    </section>
  );
}
