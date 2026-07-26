import { Settings2 } from "lucide-react";

import {
  Button,
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
  FieldTitle,
  FormSection,
  Input,
  RadioCard,
} from "@agh/ui";

import {
  RuntimeSelector,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";

import {
  AGENT_CREATE_PERMISSION_OPTIONS,
  type AgentCreatePermissionChoice,
} from "../lib/agent-create-draft";
import { inheritedAgentRuntimeFields } from "../lib/agent-effective-runtime";
import type { AgentSettingsDraft, AgentSettingsValidation } from "../lib/agent-settings-draft";
import type { AgentPayload } from "../types";

const PERMISSION_DESCRIPTIONS: Record<AgentCreatePermissionChoice, string> = {
  "": "Use the runtime's default approval mode.",
  "deny-all": "Ask before every tool call.",
  "approve-reads": "Auto-approve read-only tools; ask for the rest.",
  "approve-all": "Auto-approve every allowed tool call.",
};

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
  modelCatalogLoaded: boolean;
  modelCatalogRefreshing: boolean;
  modelCatalogError: string | null;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
}

export function AgentSettingsRuntimeSection({
  draft,
  agent,
  errors,
  disabled,
  readOnly,
  onPatch,
  providerOptions,
  providersLoading,
  runtimeModels,
  modelCatalogLoading,
  modelCatalogLoaded,
  modelCatalogRefreshing,
  modelCatalogError,
  onRefreshCatalog,
  onOpenProviderSettings,
}: AgentSettingsRuntimeSectionProps) {
  const effectiveRuntime = agent.effective_runtime;
  const effectiveProvider = effectiveRuntime?.provider?.trim() ?? "";
  const selectedProvider = draft.provider.trim() || effectiveProvider;
  const providerMatchesEffective = selectedProvider === effectiveProvider;
  const selectedModel =
    draft.model.trim() || (providerMatchesEffective ? (effectiveRuntime?.model?.trim() ?? "") : "");
  const runtimeValue: RuntimeSelectorValue = {
    provider: selectedProvider,
    model: selectedModel,
    reasoning_effort:
      draft.reasoningEffort ||
      (providerMatchesEffective && selectedModel === effectiveRuntime?.model?.trim()
        ? (effectiveRuntime?.reasoning_effort ?? "")
        : ""),
  };
  const inheritedFields = inheritedAgentRuntimeFields(agent);
  const hasRuntimeOverride = Boolean(
    draft.provider.trim() || draft.model.trim() || draft.reasoningEffort
  );

  return (
    <FormSection
      data-testid="agent-settings-runtime"
      icon={Settings2}
      title="Runtime"
      description="Provider and optional overrides for new sessions."
    >
      <Field data-invalid={Boolean(errors.provider)}>
        <FieldTitle id="agent-settings-runtime-label">Runtime</FieldTitle>
        <FieldDescription>
          Provider, model, and reasoning effort inherited by new sessions.
        </FieldDescription>
        {inheritedFields.length > 0 ? (
          <p className="text-form-hint text-info" data-testid="agent-settings-runtime-inherited">
            Inheriting {inheritedFields.join(", ")} from project runtime defaults. A selection here
            creates an agent override.
          </p>
        ) : null}
        <RuntimeSelector
          value={runtimeValue}
          onChange={next =>
            readOnly
              ? undefined
              : onPatch({
                  provider: next.provider,
                  model: next.model,
                  reasoningEffort: next.reasoning_effort,
                })
          }
          providers={providerOptions}
          models={runtimeModels}
          loading={modelCatalogLoading}
          catalogLoaded={modelCatalogLoaded}
          refreshing={modelCatalogRefreshing}
          onRefreshCatalog={onRefreshCatalog}
          onOpenProviderSettings={onOpenProviderSettings}
          disabled={disabled || providersLoading || providerOptions.length === 0}
          readOnly={readOnly}
          ariaLabelledby="agent-settings-runtime-label"
          triggerId="agent-settings-runtime-trigger"
          triggerTestId="agent-settings-runtime-select"
        />
        {hasRuntimeOverride && !readOnly ? (
          <Button
            className="mt-2"
            data-testid="agent-settings-runtime-use-project-defaults"
            disabled={disabled}
            onClick={() => onPatch({ provider: "", model: "", reasoningEffort: "" })}
            size="sm"
            type="button"
            variant="ghost"
          >
            Use project defaults
          </Button>
        ) : null}
        <FieldError data-testid="agent-settings-provider-error">{errors.provider}</FieldError>
        {modelCatalogError ? (
          <p className="text-small-body text-warning" data-testid="agent-settings-model-error">
            {modelCatalogError}
          </p>
        ) : null}
      </Field>

      <Field>
        <FieldLabel htmlFor="agent-settings-command">Command</FieldLabel>
        <FieldDescription>Optional provider command override for this agent.</FieldDescription>
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

      <Field data-invalid={Boolean(errors.permissions)}>
        <FieldLabel id="agent-settings-permissions-label">Permissions</FieldLabel>
        <FieldDescription>Default approval posture for this agent.</FieldDescription>
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
              description={PERMISSION_DESCRIPTIONS[option.value]}
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
        <FieldError data-testid="agent-settings-permissions-error">{errors.permissions}</FieldError>
      </Field>
    </FormSection>
  );
}
