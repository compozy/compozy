import { Bot, Check, Clock, Eye, Filter, Webhook } from "lucide-react";
import { useState } from "react";

import {
  Alert,
  AlertDescription,
  Button,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogToolbar,
  Field,
  FieldLabel,
  FormSection,
  Input,
} from "@agh/ui";

import type { AgentPayload } from "@/systems/agent";
import { ScopeSelector } from "@/systems/workspace";

import { useAutomationTriggerForm } from "../hooks/use-automation-trigger-form";
import type { WorkspaceOption } from "../lib/trigger-preview";
import type { CreateAutomationTriggerRequest } from "../types";
import { EventCatalog } from "./trigger-form/event-catalog";
import { FilterConditions } from "./trigger-form/filter-conditions";
import { TriggerPreview } from "./trigger-form/preview/trigger-preview";
import { ReliabilitySection } from "./trigger-form/reliability-section";
import { TriggerTargetStep } from "./trigger-form/target-step";

interface AutomationTriggerFormProps {
  activeWorkspaceId?: string | null;
  draft: CreateAutomationTriggerRequest;
  isPending: boolean;
  mode: "create" | "edit";
  onCancel: () => void;
  onChange: (draft: CreateAutomationTriggerRequest) => void;
  onSubmit: () => void;
  submitError?: string | null;
  /** Workspaces selectable for a workspace-scoped trigger. */
  workspaces?: ReadonlyArray<WorkspaceOption>;
  /** Agent catalog for the searchable selector. */
  agents?: AgentPayload[];
  agentsLoading?: boolean;
  agentsError?: string | null;
}

const EMPTY_AGENTS: AgentPayload[] = [];

export function AutomationTriggerForm({
  activeWorkspaceId,
  draft,
  isPending,
  mode,
  onCancel,
  onChange,
  onSubmit,
  submitError,
  workspaces,
  agents = EMPTY_AGENTS,
  agentsLoading = false,
  agentsError = null,
}: AutomationTriggerFormProps) {
  const form = useAutomationTriggerForm({
    activeWorkspaceId,
    draft,
    isPending,
    mode,
    onChange,
    onSubmit,
    workspaces,
  });
  // View state, not draft state: `AutomationEditorDialog` unmounts the content
  // on close, so the initial value is the whole reset.
  const [view, setView] = useState<"form" | "preview">("form");
  // Held above the view swap: the preview unmounts the form body, and a
  // disclosure someone opened must survive a look at the preview.
  const [reliabilityOpen, setReliabilityOpen] = useState(form.reliabilityDefaultOpen);

  return (
    <form
      className="flex min-h-0 flex-col"
      data-testid="automation-trigger-form"
      onSubmit={form.handleSubmit}
    >
      <EntityDialogToolbar
        trailing={
          <ScopeSelector
            ariaLabel="Trigger scope"
            disabled={mode === "edit"}
            onScopeChange={form.onScopeChange}
            onWorkspaceChange={form.onWorkspaceChange}
            scope={draft.scope}
            testIdPrefix="trigger"
            workspaceDisabled={form.isWebhook}
            workspaceId={draft.workspace_id}
            workspaces={form.resolvedWorkspaces}
          />
        }
        trailingLabel="Scope"
      />

      <EntityDialogBody data-testid="automation-trigger-form-body">
        {view === "preview" ? (
          <TriggerPreview preview={form.preview} />
        ) : (
          <>
            {submitError ? (
              <Alert className="mb-4" role="alert" variant="danger">
                <AlertDescription>{submitError}</AlertDescription>
              </Alert>
            ) : null}
            {form.isWebhook ? (
              <Alert className="mb-4" data-testid="trigger-webhook-scope-note" variant="neutral">
                <Webhook aria-hidden="true" className="size-4" />
                <AlertDescription>
                  Webhook triggers are always global; they aren&apos;t tied to a workspace.
                </AlertDescription>
              </Alert>
            ) : null}
            {form.preview.targetIssue ? (
              // With the preview closed this is the only visible reason the
              // primary is disabled — never let it live solely in the preview.
              <Alert className="mb-4" data-testid="trigger-form-blocked" variant="warning">
                <AlertDescription>{form.preview.targetIssue}</AlertDescription>
              </Alert>
            ) : null}
            <Field>
              <FieldLabel htmlFor="trigger-name">Trigger name</FieldLabel>
              <Input
                className="font-mono"
                data-testid="trigger-name-input"
                id="trigger-name"
                onChange={event => form.onName(event.target.value)}
                placeholder="summarize-failures"
                value={draft.name}
              />
            </Field>

            <FormSection
              help="The runtime event this trigger listens for. A few events need one more detail, shown right under your choice."
              icon={Clock}
              title="An event happens"
            >
              <EventCatalog
                disabledCatalogIds={form.disabledCatalogIds}
                onSelectEvent={form.onSelectEvent}
                onSubConfigChange={form.onSubConfigChange}
                selection={form.selection}
                subConfigValues={form.subConfigValues}
              />
            </FormSection>

            <FormSection
              help="Each condition is an exact match on a field from the event above. With none set, every event of that kind fires the trigger."
              icon={Filter}
              title={
                <>
                  It matches these conditions
                  <span className="ml-1.5 text-small-body font-normal text-faint">(optional)</span>
                </>
              }
            >
              <FilterConditions
                eventKind={form.eventKind}
                filter={draft.filter ?? {}}
                keyOptions={form.keyOptions}
                onChange={form.onFilterChange}
                openPayload={form.openPayload}
              />
            </FormSection>

            <FormSection
              help="Run an agent with a prompt rendered from the event, or start a Loop with typed inputs."
              icon={Bot}
              title="Run an agent, or a Loop"
            >
              <TriggerTargetStep
                catalog={form.loopCatalog}
                editorMode={mode}
                mode={form.targetMode}
                onModeChange={form.onTargetModeChange}
                agent={draft.agent_name}
                agents={agents}
                agentsError={agentsError}
                agentsLoading={agentsLoading}
                prompt={draft.prompt}
                variables={form.variables}
                onAgentChange={form.onAgentChange}
                onPromptChange={form.onPromptChange}
                loopTarget={form.loopTarget}
                onLoopTargetChange={form.onLoopTargetChange}
              />
            </FormSection>

            <ReliabilitySection
              badge={form.preview.reliabilityBadge}
              defaultOpen={form.reliabilityDefaultOpen}
              onOpenChange={setReliabilityOpen}
              open={reliabilityOpen}
              enabled={draft.enabled ?? true}
              fireLimit={draft.fire_limit ?? undefined}
              onEnabledChange={form.onEnabledChange}
              onFireLimitChange={form.onFireLimitChange}
              onRetryChange={form.onRetryChange}
              retry={form.retry}
            />
          </>
        )}
      </EntityDialogBody>

      <EntityDialogFooter
        hint={
          submitError ? undefined : (
            <>
              Created as a <b className="font-medium text-muted">dynamic</b> trigger: editable and
              deletable anytime.
            </>
          )
        }
        isSaving={isPending}
        leading={
          <Button
            aria-pressed={view === "preview"}
            data-testid="trigger-preview-toggle"
            onClick={() => setView(current => (current === "form" ? "preview" : "form"))}
            size="sm"
            type="button"
            variant="ghost"
          >
            <Eye aria-hidden="true" className="size-3.5" />
            {view === "preview" ? "Back to form" : "Show live preview"}
          </Button>
        }
        onCancel={onCancel}
        primaryDisabled={!form.canSubmit}
        primaryIcon={mode === "create" ? Check : undefined}
        primaryLabel={
          isPending ? "Saving..." : mode === "create" ? "Create trigger" : "Save changes"
        }
        primaryTestId="submit-trigger-form"
        primaryType="submit"
      />
    </form>
  );
}
