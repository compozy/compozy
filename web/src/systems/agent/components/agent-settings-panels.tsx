import {
  ActionResultBanner,
  Button,
  Field,
  FormSection,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
} from "@compozy/ui";

import type { AgentSettingsDraft, AgentSettingsValidation } from "../lib/agent-settings-draft";
import type { AgentSettingsSection } from "../lib/agent-settings-search";
import type { AgentPayload } from "../types";
import type { AgentSettingsEditorState } from "../stores/agent-settings-editor-store";
import { AgentSettingsAccessSection } from "./agent-settings-access-section";
import { AgentSettingsBasicsSection } from "./agent-settings-basics-section";
import { AgentSettingsDangerSection } from "./agent-settings-danger-section";
import { AgentSettingsInstructionsSection } from "./agent-settings-instructions-section";
import { AgentSettingsMcpSection } from "./agent-settings-mcp-section";
import { AgentSettingsRuntimeSection } from "./agent-settings-runtime-section";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";

export interface AgentSettingsPanelsProps {
  section: AgentSettingsSection;
  agent: AgentPayload;
  draft: AgentSettingsDraft;
  validation: AgentSettingsValidation | null;
  phase: AgentSettingsEditorState["phase"];
  error: string | null;
  onReloadAndRetry: () => void;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
  onDelete: () => void;
  isDeleting: boolean;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
}

export function AgentSettingsPanels(props: AgentSettingsPanelsProps) {
  const { section, agent, draft, validation } = props;
  const errors = validation?.fields ?? {};
  const disabled = props.phase === "saving";
  const fieldsReadOnly = props.phase === "denied";

  return (
    <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-5">
      <AgentSettingsBanners {...props} />
      {section === "basics" ? (
        <>
          <AgentSettingsBasicsSection
            draft={draft}
            errors={errors}
            disabled={disabled}
            readOnly={fieldsReadOnly}
            onPatch={props.onPatch}
          />
          <AgentSettingsDangerSection
            agent={agent}
            onDelete={props.onDelete}
            isDeleting={props.isDeleting}
            disabled={disabled || fieldsReadOnly}
          />
        </>
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
    </div>
  );
}

function AgentSettingsBanners(props: AgentSettingsPanelsProps) {
  return (
    <>
      {props.phase === "denied" ? (
        <ActionResultBanner
          tone="destructive"
          title="You can't edit this agent"
          description="You don't have permission to edit this agent. Ask the person who runs CompozyOS for access."
          data-testid="agent-settings-mutation-denied"
        />
      ) : null}
      {props.phase === "conflict" && props.error ? (
        <ActionResultBanner
          tone="warning"
          title="Definition conflict"
          description={props.error}
          actions={
            <Button
              type="button"
              size="sm"
              variant="secondary"
              onClick={props.onReloadAndRetry}
              data-testid="agent-settings-reload-retry"
            >
              Reload and retry
            </Button>
          }
          data-testid="agent-settings-conflict-banner"
        />
      ) : null}
      {props.phase === "failed" && props.error ? (
        <ActionResultBanner
          tone="destructive"
          title="Couldn't save agent"
          description={props.error}
          data-testid="agent-settings-save-error"
        />
      ) : null}
    </>
  );
}

type SettingsControl = "input" | "select" | "textarea";

const SETTINGS_CONTROL_CLASS: Record<SettingsControl, string> = {
  input: "h-input",
  select: "h-button-lg w-56",
  textarea: "h-72",
};

/** The fields each section lays out, by control, so the skeleton holds the section's rough height. */
const SETTINGS_SKELETON_FIELDS: Record<
  AgentSettingsSection,
  readonly { id: string; control: SettingsControl }[]
> = {
  basics: [
    { id: "name", control: "input" },
    { id: "group", control: "input" },
  ],
  runtime: [
    { id: "model", control: "select" },
    { id: "command", control: "input" },
    { id: "permissions", control: "input" },
  ],
  instructions: [{ id: "prompt", control: "textarea" }],
  access: [
    { id: "tools", control: "input" },
    { id: "blocked", control: "input" },
    { id: "groups", control: "input" },
    { id: "blocked-groups", control: "input" },
  ],
  mcp: [
    { id: "first", control: "input" },
    { id: "second", control: "input" },
  ],
};

/**
 * The settings body while the agent loads: the panel scroller and a form section with the field
 * rhythm of the open section. The section nav beside it stays real.
 */
export function AgentSettingsPanelsSkeleton({ section }: { section: AgentSettingsSection }) {
  return (
    <SkeletonRegion
      className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-5 py-5"
      data-testid="agent-settings-loading"
      label="Loading agent settings"
    >
      {/* The wrapper keeps the form section the first child of its parent, as it is when loaded:
          it drops its top rule and margin only then, and the region's own label comes first. */}
      <div>
        <FormSection title={<SkeletonText className="w-24" />}>
          {SETTINGS_SKELETON_FIELDS[section].map(field => (
            <Field key={field.id}>
              <span className="text-form-label">
                <SkeletonText className="w-16" />
              </span>
              <Skeleton className={SETTINGS_CONTROL_CLASS[field.control]} />
            </Field>
          ))}
        </FormSection>
      </div>
    </SkeletonRegion>
  );
}
