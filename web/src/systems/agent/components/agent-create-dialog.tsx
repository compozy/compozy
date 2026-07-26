import { Bot, Box, Globe } from "lucide-react";
import { type FormEvent } from "react";

import {
  Alert,
  AlertDescription,
  Dialog,
  DialogContent,
  dialogShellClass,
  EntityDialogBody,
  EntityDialogFooter,
  EntityDialogHeader,
  EntityModeToolbar,
  PillGroup,
  type EntityMode,
} from "@agh/ui";

import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import { WorkspaceCommandSelect, type WorkspaceCommandSelectOption } from "@/systems/workspace";

import { updateAgentCreateScope, type AgentCreateDialogDraft } from "../lib/agent-create-draft";
import { useAgentCreateDialogViewState } from "../hooks/use-agent-create-dialog-view-state";
import { AgentCreateDefinitionSection } from "./agent-create-definition-section";
import { AgentCreatePermissionsSection } from "./agent-create-permissions-section";
import { AgentCreateRuntimeDetailsSection } from "./agent-create-runtime-details-section";
import { AgentCreateRuntimeFields } from "./agent-create-runtime-fields";
import { AgentCreateToolsSection } from "./agent-create-tools-section";

interface AgentCreateDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  draft: AgentCreateDialogDraft;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  onSubmit: () => void;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  providersError: string | null;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogLoaded: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  submitError: string | null;
  isSubmitting: boolean;
  hasActiveWorkspace: boolean;
  workspaceId: string | null;
  workspaceName: string | null;
  initialMode?: EntityMode;
}

/**
 * Create-only agent editor on the shared entity-dialog shell.
 *
 * Agent edit stays on `agents.$name.settings` (D3) — this dialog never mutates
 * an existing definition, so it carries no dirty gate and no expected-digest
 * round trip.
 */
function AgentCreateDialog({
  open,
  onOpenChange,
  draft,
  onDraftChange,
  onSubmit,
  onRefreshCatalog,
  onOpenProviderSettings,
  providerOptions,
  providersLoading,
  providersError,
  runtimeModels,
  modelCatalogLoading,
  modelCatalogLoaded,
  modelCatalogRefreshing,
  modelCatalogError,
  submitError,
  isSubmitting,
  hasActiveWorkspace,
  workspaceId,
  workspaceName,
  initialMode = "simple",
}: AgentCreateDialogProps) {
  const { handleOpenChange, mode, revealAdvancedWhenBlocked, setMode, validation, visibleErrors } =
    useAgentCreateDialogViewState({
      draft,
      hasActiveWorkspace,
      initialMode,
      onOpenChange,
      open,
      providerOptions,
      providersError,
      providersLoading,
    });

  // The write target is the active workspace: the hook resolves workspace
  // providers from it, so the selector reports where the definition lands
  // rather than offering a retarget the create path cannot honour.
  const workspaceOptions: WorkspaceCommandSelectOption[] =
    workspaceId && workspaceName ? [{ id: workspaceId, name: workspaceName }] : [];

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (isSubmitting) return;
    // An advanced-only field can only be fixed where it renders. Reveal it
    // instead of leaving a disabled primary with no visible cause.
    if (revealAdvancedWhenBlocked()) return;
    if (!validation.canSubmit) return;
    onSubmit();
  };

  return (
    <Dialog onOpenChange={handleOpenChange} open={open}>
      <DialogContent
        className={`grid-rows-[auto_auto_minmax(0,1fr)_auto] text-fg ${dialogShellClass("lg")}`}
        data-testid="agent-create-dialog"
        showCloseButton={false}
        unframed
      >
        <EntityDialogHeader
          description={
            <>
              An agent is a reusable <b className="font-medium text-muted">definition</b> —
              instructions plus a runtime. Sessions launch from it and inherit its access policy.
            </>
          }
          eyebrow="Operate · Agent"
          icon={Bot}
          onClose={isSubmitting ? undefined : () => handleOpenChange(false)}
          title="Create agent"
        />

        <EntityModeToolbar
          mode={mode}
          onModeChange={setMode}
          testIdPrefix="agent-create"
          trailing={
            <div className="flex min-w-0 flex-wrap items-center gap-2">
              <PillGroup
                aria-label="Scope"
                data-testid="agent-create-scope-group"
                items={[
                  {
                    value: "global",
                    label: (
                      <span className="flex items-center gap-1.5">
                        <Globe aria-hidden="true" className="size-3" />
                        Global
                      </span>
                    ),
                    disabled: isSubmitting,
                    testId: "agent-create-scope-global",
                  },
                  {
                    value: "workspace",
                    label: (
                      <span className="flex items-center gap-1.5">
                        <Box aria-hidden="true" className="size-3" />
                        Workspace
                      </span>
                    ),
                    disabled: isSubmitting || !hasActiveWorkspace,
                    testId: "agent-create-scope-workspace",
                  },
                ]}
                onChange={next => onDraftChange(updateAgentCreateScope(draft, next))}
                size="md"
                value={draft.scope}
              />
              {draft.scope === "workspace" && hasActiveWorkspace ? (
                <div className="min-w-40 w-auto max-w-xs shrink-0">
                  <WorkspaceCommandSelect
                    disabled
                    onChange={() => undefined}
                    size="compact"
                    testIdPrefix="agent-create-workspace"
                    triggerTestId="agent-create-workspace-select"
                    value={workspaceId ?? null}
                    workspaces={workspaceOptions}
                  />
                </div>
              ) : null}
            </div>
          }
          trailingLabel="Scope"
        />

        <form className="contents" onSubmit={handleSubmit}>
          <EntityDialogBody className="flex flex-col">
            {submitError || visibleErrors.scope ? (
              <Alert className="mb-4" data-testid="agent-create-submit-error" variant="danger">
                <AlertDescription>{submitError ?? visibleErrors.scope}</AlertDescription>
              </Alert>
            ) : null}

            <AgentCreateDefinitionSection
              draft={draft}
              errors={visibleErrors}
              onDraftChange={onDraftChange}
            >
              <AgentCreateRuntimeFields
                draft={draft}
                errors={visibleErrors}
                modelCatalogError={modelCatalogError}
                modelCatalogLoaded={modelCatalogLoaded}
                modelCatalogLoading={modelCatalogLoading}
                modelCatalogRefreshing={modelCatalogRefreshing}
                onDraftChange={onDraftChange}
                onOpenProviderSettings={onOpenProviderSettings}
                onRefreshCatalog={onRefreshCatalog}
                providerOptions={providerOptions}
                providersLoading={providersLoading}
                runtimeModels={runtimeModels}
              />
            </AgentCreateDefinitionSection>

            {mode === "advanced" ? (
              <>
                <AgentCreatePermissionsSection draft={draft} onDraftChange={onDraftChange} />
                <AgentCreateRuntimeDetailsSection draft={draft} onDraftChange={onDraftChange} />
                <AgentCreateToolsSection
                  draft={draft}
                  errors={visibleErrors}
                  onDraftChange={onDraftChange}
                />
              </>
            ) : null}
          </EntityDialogBody>

          <EntityDialogFooter
            cancelDisabled={isSubmitting}
            cancelTestId="agent-create-cancel"
            hint="The definition is validated by the daemon before sessions can launch from it."
            isSaving={isSubmitting}
            onCancel={() => handleOpenChange(false)}
            primaryDisabled={
              isSubmitting || (mode === "simple" ? !validation.simpleValid : !validation.canSubmit)
            }
            primaryLabel={isSubmitting ? "Creating…" : "Create agent"}
            primaryTestId="submit-agent-create"
            primaryType="submit"
          />
        </form>
      </DialogContent>
    </Dialog>
  );
}

export { AgentCreateDialog };
export type { AgentCreateDialogProps };
