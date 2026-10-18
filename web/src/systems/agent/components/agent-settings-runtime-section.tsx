import { Settings2 } from "lucide-react";

import {
  Button,
  Field,
  FieldDescription,
  FieldError,
  FieldHeader,
  FieldLabel,
  FieldTitle,
  FormSection,
  HelpTip,
  Input,
  RadioCard,
} from "@compozy/ui";

import { AGENT_CREATE_PERMISSION_OPTIONS } from "../lib/agent-permissions";
import {
  hasAgentRuntimeOverride,
  inheritedAgentRuntimeFields,
} from "../lib/agent-effective-runtime";
import {
  agentSettingsRuntimeSpeed,
  agentSettingsRuntimeValue,
  type AgentSettingsDraft,
  type AgentSettingsValidation,
} from "../lib/agent-settings-draft";
import type { AgentPayload } from "../types";
import {
  type RuntimeModelOption,
  type RuntimeProviderOption,
  RuntimeSelector,
} from "@/systems/runtime";

export interface AgentSettingsRuntimeSectionProps {
  draft: AgentSettingsDraft;
  agent: AgentPayload;
  errors: AgentSettingsValidation["fields"];
  disabled: boolean;
  readOnly: boolean;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  runtimeModels: RuntimeModelOption[];
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
}

const CLEARED_RUNTIME_OVERRIDE: Partial<AgentSettingsDraft> = {
  provider: "",
  model: "",
  reasoningEffort: "",
  speed: "",
  acpOptions: [],
};

type AgentSettingsModelFieldProps = Omit<AgentSettingsRuntimeSectionProps, "errors"> & {
  error: string | undefined;
};

function AgentSettingsModelField({
  draft,
  agent,
  error,
  disabled,
  readOnly,
  onPatch,
  providerOptions,
  providersLoading,
  runtimeModels,
  modelCatalogLoading,
  modelCatalogRefreshing,
  modelCatalogError,
  onRefreshCatalog,
  onOpenProviderSettings,
}: AgentSettingsModelFieldProps) {
  const inheritedFields = inheritedAgentRuntimeFields(agent);
  const patch = (next: Partial<AgentSettingsDraft>) => {
    if (!readOnly) onPatch(next);
  };

  return (
    <Field data-invalid={Boolean(error)}>
      <FieldHeader>
        <FieldTitle id="agent-settings-runtime-label">Model</FieldTitle>
        <HelpTip label="About model">
          Provider, model, Reasoning, Fast, and advanced options inherited by new sessions.
        </HelpTip>
      </FieldHeader>
      {inheritedFields.length > 0 ? (
        <FieldDescription data-testid="agent-settings-runtime-inherited">
          Using the project's default {inheritedFields.join(", ")}. Pick a different one to override
          it for this agent.
        </FieldDescription>
      ) : null}
      <RuntimeSelector
        value={agentSettingsRuntimeValue(agent, draft)}
        onChange={(next, normalizedSpeed) =>
          patch({
            provider: next.provider,
            model: next.model,
            reasoningEffort: next.reasoning_effort,
            acpOptions: next.acp_options ?? [],
            ...(normalizedSpeed ? { speed: normalizedSpeed } : {}),
          })
        }
        providers={providerOptions}
        models={runtimeModels}
        loading={modelCatalogLoading}
        refreshing={modelCatalogRefreshing}
        onRefreshCatalog={onRefreshCatalog}
        onOpenProviderSettings={onOpenProviderSettings}
        disabled={disabled || providersLoading || providerOptions.length === 0}
        readOnly={readOnly}
        speed={agentSettingsRuntimeSpeed(agent, draft)}
        onSpeedChange={speed => patch({ speed })}
        ariaLabelledby="agent-settings-runtime-label"
        triggerId="agent-settings-runtime-trigger"
        triggerTestId="agent-settings-runtime-select"
      />
      {hasAgentRuntimeOverride(draft) && !readOnly ? (
        <Button
          className="mt-2"
          data-testid="agent-settings-runtime-use-project-defaults"
          disabled={disabled}
          onClick={() => onPatch(CLEARED_RUNTIME_OVERRIDE)}
          size="sm"
          type="button"
          variant="link"
        >
          Use project defaults
        </Button>
      ) : null}
      <FieldError data-testid="agent-settings-provider-error">{error}</FieldError>
      {modelCatalogError ? (
        <p className="text-small-body text-warning" data-testid="agent-settings-model-error">
          {modelCatalogError}
        </p>
      ) : null}
    </Field>
  );
}

interface AgentSettingsFieldProps {
  draft: AgentSettingsDraft;
  disabled: boolean;
  readOnly: boolean;
  onPatch: (patch: Partial<AgentSettingsDraft>) => void;
}

function AgentSettingsCommandField({
  draft,
  disabled,
  readOnly,
  onPatch,
}: AgentSettingsFieldProps) {
  return (
    <Field>
      <FieldHeader>
        <FieldLabel htmlFor="agent-settings-command">Command</FieldLabel>
        <HelpTip label="About command">
          The program CompozyOS starts for this agent's provider. Leave it empty to use the default.
        </HelpTip>
      </FieldHeader>
      <Input
        id="agent-settings-command"
        data-testid="agent-settings-command"
        className="font-mono"
        value={draft.command}
        disabled={disabled}
        readOnly={readOnly}
        aria-disabled={readOnly || undefined}
        onChange={event => onPatch({ command: event.target.value })}
        placeholder="Leave blank to use the provider default"
      />
    </Field>
  );
}

function AgentSettingsPermissionsField({
  draft,
  error,
  disabled,
  readOnly,
  onPatch,
}: AgentSettingsFieldProps & { error: string | undefined }) {
  return (
    <Field data-invalid={Boolean(error)}>
      <FieldLabel id="agent-settings-permissions-label">Permissions</FieldLabel>
      {draft.legacyPermissions ? (
        <p
          className="mb-2 text-small-body text-warning"
          data-testid="agent-settings-permissions-legacy"
        >
          Unrecognized permission mode: {draft.legacyPermissions}
        </p>
      ) : null}
      <div
        aria-labelledby="agent-settings-permissions-label"
        className="grid gap-2 sm:grid-cols-2"
        data-testid="agent-settings-permissions"
        role="radiogroup"
      >
        {AGENT_CREATE_PERMISSION_OPTIONS.map(option => (
          <RadioCard
            key={option.value || "inherit"}
            data-testid={`agent-settings-permissions-${option.value || "inherit"}`}
            description={option.description}
            disabled={disabled}
            aria-disabled={readOnly || undefined}
            onSelect={() => {
              if (readOnly) return;
              onPatch({
                permissions: option.value,
                legacyPermissions: null,
              });
            }}
            selected={draft.permissions === option.value}
            title={option.label}
          />
        ))}
      </div>
      <FieldError data-testid="agent-settings-permissions-error">{error}</FieldError>
    </Field>
  );
}

export function AgentSettingsRuntimeSection({
  errors,
  ...props
}: AgentSettingsRuntimeSectionProps) {
  return (
    <FormSection data-testid="agent-settings-runtime" icon={Settings2} title="Model">
      <AgentSettingsModelField {...props} error={errors.provider} />
      <AgentSettingsCommandField {...props} />
      <AgentSettingsPermissionsField {...props} error={errors.permissions} />
    </FormSection>
  );
}
