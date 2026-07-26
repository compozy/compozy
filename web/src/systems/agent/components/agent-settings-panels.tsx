import { ActionResultBanner, Button } from "@agh/ui";

import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

import type { AgentSettingsDraft, AgentSettingsValidation } from "../lib/agent-settings-draft";
import type { AgentSettingsSection } from "../lib/agent-settings-search";
import type { AgentPayload } from "../types";
import { AgentSettingsAccessSection } from "./agent-settings-access-section";
import { AgentSettingsBasicsSection } from "./agent-settings-basics-section";
import { AgentSettingsDangerSection } from "./agent-settings-danger-section";
import { AgentSettingsInstructionsSection } from "./agent-settings-instructions-section";
import { AgentSettingsMcpSection } from "./agent-settings-mcp-section";
import { AgentSettingsRuntimeSection } from "./agent-settings-runtime-section";

export interface AgentSettingsPanelsProps {
  section: AgentSettingsSection;
  agent: AgentPayload;
  draft: AgentSettingsDraft;
  validation: AgentSettingsValidation | null;
  disabled: boolean;
  readOnly: boolean;
  mutationDenied: boolean;
  saveError: string | null;
  conflictBanner: string | null;
  onReloadAndRetry: () => void;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
  onDelete: () => void;
  isDeleting: boolean;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogLoaded: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
}

export function AgentSettingsPanels(props: AgentSettingsPanelsProps) {
  const { section, agent, draft, validation, disabled, readOnly } = props;
  const errors = validation?.fields ?? {};
  const fieldsReadOnly = readOnly || props.mutationDenied;

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto py-5">
      <AgentSettingsBanners {...props} />
      {section === "basics" ? (
        <AgentSettingsBasicsSection
          draft={draft}
          errors={errors}
          disabled={disabled}
          readOnly={fieldsReadOnly}
          onPatch={props.onPatch}
        />
      ) : null}
      {section === "runtime" ? (
        <AgentSettingsRuntimeSection
          agent={agent}
          draft={draft}
          errors={errors}
          disabled={disabled}
          readOnly={fieldsReadOnly}
          onPatch={props.onPatch}
          providerOptions={props.providerOptions}
          providersLoading={props.providersLoading}
          runtimeModels={props.runtimeModels}
          modelCatalogLoading={props.modelCatalogLoading}
          modelCatalogLoaded={props.modelCatalogLoaded}
          modelCatalogRefreshing={props.modelCatalogRefreshing}
          modelCatalogError={props.modelCatalogError}
          onRefreshCatalog={props.onRefreshCatalog}
          onOpenProviderSettings={props.onOpenProviderSettings}
        />
      ) : null}
      {section === "instructions" ? (
        <AgentSettingsInstructionsSection
          draft={draft}
          errors={errors}
          disabled={disabled}
          readOnly={fieldsReadOnly}
          onPatch={props.onPatch}
        />
      ) : null}
      {section === "access" ? (
        <AgentSettingsAccessSection
          draft={draft}
          disabled={disabled}
          readOnly={fieldsReadOnly}
          onPatch={props.onPatch}
        />
      ) : null}
      {section === "mcp" ? <AgentSettingsMcpSection agent={agent} /> : null}
      {section === "danger" ? (
        <AgentSettingsDangerSection
          agent={agent}
          onDelete={props.onDelete}
          isDeleting={props.isDeleting}
          disabled={disabled || fieldsReadOnly}
        />
      ) : null}
    </div>
  );
}

function AgentSettingsBanners(props: AgentSettingsPanelsProps) {
  return (
    <>
      {props.mutationDenied ? (
        <ActionResultBanner
          tone="danger"
          title="Editing requires an operator token"
          description="Add an operator token, then reload this page to edit the definition."
          data-testid="agent-settings-mutation-denied"
        />
      ) : null}
      {props.conflictBanner ? (
        <ActionResultBanner
          tone="warning"
          title="Definition conflict"
          description={props.conflictBanner}
          actions={
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={props.onReloadAndRetry}
              data-testid="agent-settings-reload-retry"
            >
              Reload and retry
            </Button>
          }
          data-testid="agent-settings-conflict-banner"
        />
      ) : null}
      {props.saveError ? (
        <ActionResultBanner
          tone="danger"
          title="Couldn't save agent"
          description={props.saveError}
          data-testid="agent-settings-save-error"
        />
      ) : null}
    </>
  );
}
