import { useEffect, useEffectEvent, useLayoutEffect, useState } from "react";
import { useSelector } from "@xstate/store-react";
import { useStoreBinding } from "@/hooks/use-store-binding";

import { AutomationApiError } from "../adapters/automation-api";
import {
  automationJobToFormDraft,
  automationTriggerToFormDraft,
  buildAutomationFormRequest,
  createAutomationFormDraft,
  type AutomationEditorSection,
  type AutomationFormDraft,
  type CreateAutomationFormDraftOptions,
} from "../lib/automation-form-draft";
import { createAutomationDialogHandle } from "../lib/dialog-handle";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type { AutomationJob, AutomationTrigger } from "../types";
import {
  useCreateAutomationJob,
  useCreateAutomationTrigger,
  useUpdateAutomationJob,
  useUpdateAutomationTrigger,
} from "./use-automation-actions";
import { createWorkspaceEditorLogic, type WorkspaceEditorFailure } from "./automation-editor-store";
import { useAgents } from "@/systems/agent";

/** What the save produced: the daemon entity decides the detail route. */
export type AutomationSaveResult =
  | { entity: "job"; automation: AutomationJob }
  | { entity: "trigger"; automation: AutomationTrigger };

interface AutomationEditorShared {
  draft: AutomationFormDraft;
  /** Opens with this section in view (the detail "Set up retries" opens Options). */
  section?: AutomationEditorSection;
}

export type AutomationEditorState =
  | (AutomationEditorShared & {
      mode: "create";
      /** Does = Start a Loop is fixed to the seeded Loop (Loop page "Automate"). */
      lockedLoop?: string;
    })
  | (AutomationEditorShared & {
      mode: "edit";
      entity: "job" | "trigger";
      id: string;
      profile: string;
    });

export interface AutomationEditorCreateOptions extends CreateAutomationFormDraftOptions {
  section?: AutomationEditorSection;
}

interface AutomationEditorParams {
  activeWorkspaceId?: string | null;
  onSaved?: (result: AutomationSaveResult) => void;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

const editorLogic = createWorkspaceEditorLogic<AutomationEditorState, AutomationSaveResult>();

function agentCatalogErrorMessage(error: unknown): string | null {
  if (!(error instanceof Error)) return error ? "Unable to load agents." : null;
  return error.message.trim() || "Unable to load agents.";
}

const NAME_TAKEN = /name already exists/i;

/** A name conflict belongs to the Name field; anything else stays a dialog alert. */
export function automationSaveFailure(name: string) {
  return (error: unknown): WorkspaceEditorFailure => {
    if (
      error instanceof AutomationApiError &&
      error.status === 409 &&
      NAME_TAKEN.test(error.message)
    ) {
      const message = `An automation named ${name.trim()} already exists.`;
      return { submitError: message, submitErrorField: "name", toastError: message };
    }
    if (error instanceof AutomationApiError && error.status === 0) {
      return {
        submitError: "Couldn't save the automation. Check your connection and try again.",
        toastError: "Couldn't save the automation.",
      };
    }
    const detail =
      error instanceof Error && error.message.trim() !== ""
        ? error.message.trim()
        : "Couldn't save the automation.";
    return { submitError: detail, toastError: detail };
  };
}

/**
 * One create/edit controller for every automation. Shared by the listing
 * (create + deep links) and the detail page (edit, optionally at a section).
 * The Starts choice decides whether the save creates a job or a trigger.
 */
export function useAutomationEditor({
  activeWorkspaceId,
  onSaved,
  workspaces,
}: AutomationEditorParams) {
  const { store } = useStoreBinding(activeWorkspaceId, () =>
    editorLogic.createStore({ workspaceId: activeWorkspaceId })
  );
  const editor = useSelector(store, snapshot => snapshot.context.editor);
  const pendingRequest = useSelector(store, snapshot => snapshot.context.pendingRequest);
  const submitError = useSelector(store, snapshot => snapshot.context.submitError);
  const submitErrorField = useSelector(store, snapshot => snapshot.context.submitErrorField);
  const [handle] = useState(createAutomationDialogHandle);
  // A global automation still addresses the active project's agents.
  const agentsQuery = useAgents(activeWorkspaceId);
  const createJob = useCreateAutomationJob();
  const updateJob = useUpdateAutomationJob();
  const createTrigger = useCreateAutomationTrigger();
  const updateTrigger = useUpdateAutomationTrigger();
  const handleSaveSucceeded = useEffectEvent((result: AutomationSaveResult) => onSaved?.(result));

  useLayoutEffect(
    () => () => {
      store.trigger.lifecycleDisposed();
    },
    [store]
  );

  useEffect(() => {
    const succeeded = store.on("saveSucceeded", event => handleSaveSucceeded(event.result));
    return () => {
      succeeded.unsubscribe();
    };
  }, [store]);

  const open = (next: AutomationEditorState) =>
    store.trigger.editorOpened({ editor: next, workspaceId: activeWorkspaceId });

  const openCreate = ({ section, ...draftOptions }: AutomationEditorCreateOptions = {}) =>
    open({
      draft: createAutomationFormDraft(activeWorkspaceId, draftOptions),
      lockedLoop: draftOptions.loop,
      mode: "create",
      section,
    });

  const openEdit = (
    automation: AutomationJob | AutomationTrigger,
    { section }: { section?: AutomationEditorSection } = {}
  ) => {
    const isTrigger = "event" in automation;
    open({
      draft: isTrigger
        ? automationTriggerToFormDraft(automation)
        : automationJobToFormDraft(automation),
      entity: isTrigger ? "trigger" : "job",
      id: automation.id,
      mode: "edit",
      profile: automation.profile_name,
      section,
    });
  };

  const close = () => store.trigger.editorClosed();

  const save = async (current: AutomationEditorState): Promise<AutomationSaveResult> => {
    const request = buildAutomationFormRequest(current.draft);
    if (request.entity === "job") {
      const automation =
        current.mode === "create"
          ? await createJob.mutateAsync(request.create)
          : await updateJob.mutateAsync({
              data: request.update,
              id: current.id,
              profile: current.profile,
            });
      return { entity: "job", automation };
    }
    const automation =
      current.mode === "create"
        ? await createTrigger.mutateAsync(request.create)
        : await updateTrigger.mutateAsync({
            data: request.update,
            id: current.id,
            profile: current.profile,
          });
    return { entity: "trigger", automation };
  };

  const handleSubmit = () => {
    const current = editor;
    if (!current) return;
    store.trigger.submissionRequested({
      describeFailure: automationSaveFailure(current.draft.name),
      execute: () => save(current),
      successMessage: (mode, result) =>
        mode === "create"
          ? `Created ${result.automation.name}.`
          : `Saved ${result.automation.name}.`,
      workspaceId: activeWorkspaceId,
    });
  };

  const editorDialogProps = {
    activeWorkspaceId,
    agents: agentsQuery.data ?? [],
    agentsError: agentCatalogErrorMessage(agentsQuery.error),
    agentsLoading: agentsQuery.isLoading,
    handle,
    workspaces,
    editor: editor
      ? {
          ...editor,
          isPending: pendingRequest !== null,
          onCancel: close,
          onChange: (draft: AutomationFormDraft) =>
            store.trigger.draftChanged({ draft: { ...editor, draft } }),
          onSubmit: handleSubmit,
          submitError,
          submitErrorField,
        }
      : null,
  };

  return { close, editor, editorDialogProps, openCreate, openEdit };
}
