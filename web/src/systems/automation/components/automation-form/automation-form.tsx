import { Check, Eye } from "lucide-react";
import { useEffect, useRef, useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  EntityDialogBody,
  EntityDialogFooter,
  Field,
  FieldError,
  FieldLabel,
  Input,
} from "@compozy/ui";

import { useAutomationForm } from "../../hooks/use-automation-form";
import {
  automationFormEntity,
  type AutomationEditorSection,
  type AutomationFormDraft,
} from "../../lib/automation-form-draft";
import type { WorkspaceOption } from "../../lib/trigger-preview";
import { AutomationEditorSentenceBar } from "./automation-editor-sentence-bar";
import { AutomationFormSection } from "./automation-form-section";
import { AutomationOptions } from "./automation-options";
import { DoesSection } from "./does/does-section";
import { OnlyIfSection } from "./only-if/only-if-section";
import { AutomationPreview } from "./preview/automation-preview";
import { StartsSection } from "./starts/starts-section";
import type { AgentPayload } from "@/systems/agent";
import { ProfileDestinationChip } from "@/systems/profiles";

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

/** Name → Starts → Only if → Does → Options, with the live sentence and the preview swap. */
export function AutomationForm({
  activeWorkspaceId,
  agents = EMPTY_AGENTS,
  agentsError = null,
  agentsLoading = false,
  draft,
  isPending,
  lockedLoop,
  mode,
  onCancel,
  onChange,
  onSubmit,
  profileDestination,
  section,
  submitError,
  submitErrorField,
  workspaces,
}: AutomationFormProps) {
  const form = useAutomationForm({
    activeWorkspaceId,
    draft,
    isPending,
    lockedLoop,
    mode,
    onChange,
    onSubmit,
    workspaces,
  });
  // View state, not draft state: the dialog unmounts the content on close.
  const [view, setView] = useState<"form" | "preview">("form");
  // Held above the preview swap so a fold someone opened survives a look at the preview.
  const [optionsOpen, setOptionsOpen] = useState(form.optionsDefaultOpen || section === "options");
  const optionsRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (section === "options") optionsRef.current?.scrollIntoView?.({ block: "nearest" });
  }, [section]);

  const hasConditions = draft.start !== "schedule";
  const nameError = submitErrorField === "name" ? submitError : null;
  const dialogError = submitErrorField === "name" ? null : submitError;

  return (
    <form
      className="flex min-h-0 flex-col"
      data-entity={automationFormEntity(draft)}
      data-testid="automation-form"
      onSubmit={form.handleSubmit}
    >
      <AutomationEditorSentenceBar ready={form.ready} sentence={form.sentence} />
      <EntityDialogBody data-testid="automation-form-body">
        {view === "preview" ? (
          <AutomationPreview draft={draft} mode={mode} now={form.now} sentence={form.sentence} />
        ) : (
          <>
            <AutomationFormSection
              data-testid="automation-form-name"
              description="How you'll find it in the list"
              number={1}
              title="Name"
            >
              <Field data-invalid={nameError ? true : undefined}>
                <FieldLabel className="sr-only" htmlFor="automation-name">
                  Automation name
                </FieldLabel>
                <Input
                  aria-invalid={nameError ? true : undefined}
                  className="font-mono"
                  data-testid="automation-name-input"
                  id="automation-name"
                  onChange={event => form.onName(event.target.value)}
                  placeholder="morning-digest"
                  value={draft.name}
                />
                {nameError ? <FieldError>{nameError}</FieldError> : null}
              </Field>
            </AutomationFormSection>
            <StartsSection draft={draft} form={form} mode={mode} />
            {hasConditions ? <OnlyIfSection conditions={draft.conditions} form={form} /> : null}
            <DoesSection
              agents={agents}
              agentsError={agentsError}
              agentsLoading={agentsLoading}
              draft={draft}
              form={form}
              isPending={isPending}
              lockedLoop={lockedLoop}
              mode={mode}
              number={hasConditions ? 4 : 3}
            />
            <AutomationOptions
              draft={draft}
              form={form}
              onOpenChange={setOptionsOpen}
              open={optionsOpen}
              ref={optionsRef}
            />
          </>
        )}
      </EntityDialogBody>

      {dialogError ? (
        <div className="shrink-0 px-5 pb-4">
          <Alert data-testid="automation-form-error" role="alert" variant="danger">
            <AlertDescription>{dialogError}</AlertDescription>
          </Alert>
        </div>
      ) : null}

      <EntityDialogFooter
        hint={
          <AutomationDestinationNote
            destination={form.destination}
            mode={mode}
            profileDestination={mode === "create" ? profileDestination : null}
          />
        }
        isSaving={isPending}
        leading={
          <Button
            aria-pressed={view === "preview"}
            data-testid="automation-preview-toggle"
            onClick={() => setView(current => (current === "form" ? "preview" : "form"))}
            size="sm"
            type="button"
            variant="ghost"
          >
            <Eye aria-hidden="true" className="size-3.5" />
            {view === "preview" ? "Back to form" : "Show preview"}
          </Button>
        }
        onCancel={onCancel}
        primaryDisabled={!form.ready}
        primaryIcon={mode === "create" ? Check : undefined}
        primaryLabel={
          isPending ? "Saving…" : mode === "create" ? "Create automation" : "Save changes"
        }
        primaryTestId="automation-form-submit"
        primaryType="submit"
      />
    </form>
  );
}

/** `Creates in checkout-api.` · `Creates a Global automation.` · `Saves to checkout-api.` */
function AutomationDestinationNote({
  destination,
  mode,
  profileDestination,
}: {
  destination: string | null;
  mode: "create" | "edit";
  profileDestination?: string | null;
}) {
  const place = <span className="font-medium text-fg">{destination ?? "Global"}</span>;
  const text =
    mode === "edit" ? (
      <>Saves to {place}.</>
    ) : destination === null ? (
      <>Creates a {place} automation.</>
    ) : (
      <>Creates in {place}.</>
    );
  return (
    <span className="inline-flex max-w-full flex-wrap items-center gap-1.5">
      <span className="text-form-hint text-muted" data-testid="automation-destination">
        {text}
      </span>
      {profileDestination ? <ProfileDestinationChip profile={profileDestination} /> : null}
    </span>
  );
}
