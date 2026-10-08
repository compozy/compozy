import { useState, type RefObject } from "react";

import { EntityDialogBody } from "@compozy/ui";

import { useAutomationForm, type AutomationFormModel } from "../../hooks/use-automation-form";
import { useAutomationOptionsFold } from "../../hooks/use-automation-options-fold";
import {
  automationFormEntity,
  type AutomationEditorSection,
  type AutomationFormDraft,
} from "../../lib/automation-form-draft";
import type { WorkspaceOption } from "../../lib/trigger-preview";
import { AutomationEditorSentenceBar } from "./automation-editor-sentence-bar";
import {
  AutomationFormErrorAlert,
  AutomationFormFooter,
  AutomationNameSection,
} from "./automation-form-parts";
import { AutomationOptions } from "./automation-options";
import { DoesSection } from "./does/does-section";
import { OnlyIfSection } from "./only-if/only-if-section";
import { AutomationPreview } from "./preview/automation-preview";
import { StartsSection } from "./starts/starts-section";
import type { AgentPayload } from "@/systems/agent";

export interface AutomationFormProps {
  activeWorkspaceId?: string | null;
  agents?: AgentPayload[];
  agentsError?: string | null;
  agentsLoading?: boolean;
  draft: AutomationFormDraft;
  isPending: boolean;
  lockedLoop?: string;
  mode: "create" | "edit";
  onCancel: () => void;
  onChange: (draft: AutomationFormDraft) => void;
  onSubmit: () => void;
  /** The profile a creation lands in while the aggregate is on (ADR-005). */
  profileDestination?: string | null;
  section?: AutomationEditorSection;
  submitError?: string | null;
  submitErrorField?: "name" | null;
  workspaces?: ReadonlyArray<WorkspaceOption>;
}

const EMPTY_AGENTS: AgentPayload[] = [];

/** The name error sits on the Name field; any other save error is a dialog alert. */
function splitSubmitError(
  submitError: string | null | undefined,
  field: "name" | null | undefined
): { nameError: string | null; dialogError: string | null } {
  const error = submitError ?? null;
  return field === "name"
    ? { nameError: error, dialogError: null }
    : { nameError: null, dialogError: error };
}

/** Name → Starts → Only if → Does → Options, with the live sentence and the preview swap. */
export function AutomationForm(props: AutomationFormProps) {
  const { draft, isPending, mode, section } = props;
  const form = useAutomationForm({
    activeWorkspaceId: props.activeWorkspaceId,
    draft,
    isPending,
    lockedLoop: props.lockedLoop,
    mode,
    onChange: props.onChange,
    onSubmit: props.onSubmit,
    workspaces: props.workspaces,
  });
  // View state, not draft state: the dialog unmounts the content on close.
  const [previewing, setPreviewing] = useState(false);
  const {
    open: optionsOpen,
    ref: optionsRef,
    setOpen: setOptionsOpen,
  } = useAutomationOptionsFold(form.optionsDefaultOpen, section);
  const { nameError, dialogError } = splitSubmitError(props.submitError, props.submitErrorField);

  return (
    <form
      className="flex min-h-0 flex-col"
      data-entity={automationFormEntity(draft)}
      data-testid="automation-form"
      onSubmit={form.handleSubmit}
    >
      <AutomationEditorSentenceBar ready={form.ready} sentence={form.sentence} />
      <EntityDialogBody data-testid="automation-form-body">
        {previewing ? (
          <AutomationPreview draft={draft} mode={mode} now={form.now} sentence={form.sentence} />
        ) : (
          <AutomationFormFields
            form={form}
            formProps={props}
            nameError={nameError}
            onOptionsOpenChange={setOptionsOpen}
            optionsOpen={optionsOpen}
            optionsRef={optionsRef}
          />
        )}
      </EntityDialogBody>
      <AutomationFormErrorAlert error={dialogError} />
      <AutomationFormFooter
        destination={form.destination}
        isPending={isPending}
        mode={mode}
        onCancel={props.onCancel}
        onTogglePreview={() => setPreviewing(current => !current)}
        previewing={previewing}
        profileDestination={props.profileDestination}
        ready={form.ready}
      />
    </form>
  );
}

function AutomationFormFields({
  form,
  formProps,
  nameError,
  onOptionsOpenChange,
  optionsOpen,
  optionsRef,
}: {
  form: AutomationFormModel;
  formProps: AutomationFormProps;
  nameError: string | null;
  onOptionsOpenChange: (open: boolean) => void;
  optionsOpen: boolean;
  optionsRef: RefObject<HTMLDivElement | null>;
}) {
  const { draft, mode } = formProps;
  const hasConditions = draft.start !== "schedule";
  return (
    <>
      <AutomationNameSection error={nameError} name={draft.name} onName={form.onName} />
      <StartsSection draft={draft} form={form} mode={mode} />
      {hasConditions ? <OnlyIfSection conditions={draft.conditions} form={form} /> : null}
      <DoesSection
        agents={formProps.agents ?? EMPTY_AGENTS}
        agentsError={formProps.agentsError ?? null}
        agentsLoading={formProps.agentsLoading ?? false}
        draft={draft}
        form={form}
        isPending={formProps.isPending}
        lockedLoop={formProps.lockedLoop}
        mode={mode}
        number={hasConditions ? 4 : 3}
      />
      <AutomationOptions
        draft={draft}
        form={form}
        onOpenChange={onOptionsOpenChange}
        open={optionsOpen}
        ref={optionsRef}
      />
    </>
  );
}
