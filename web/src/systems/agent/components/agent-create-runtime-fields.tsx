import type { ComponentProps } from "react";

import {
  Button,
  cn,
  Field,
  FieldDescription,
  FieldError,
  FieldHeader,
  FieldLabel,
  HelpTip,
} from "@compozy/ui";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";
import { hasAgentRuntimeOverride } from "../lib/agent-effective-runtime";
import {
  type RuntimeModelOption,
  type RuntimeProviderOption,
  RuntimeSelector,
  type RuntimeSelectorValue,
} from "@/systems/runtime";

export interface AgentCreateRuntimeFieldsProps extends ComponentProps<"div"> {
  draft: AgentCreateDialogDraft;
  errors: Record<string, string | undefined>;
  modelCatalogError: string | null;
  modelCatalogLoading: boolean;
  modelCatalogRefreshing: boolean;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  runtimeModels: RuntimeModelOption[];
}

/**
 * Simple tier: the runtime selector.
 *
 * Catalog state stays visible rather than moving into the help tip: a Simple
 * view that hides catalog truth would let someone submit against a stale or
 * failed catalog without knowing it.
 */
export function AgentCreateRuntimeFields({
  draft,
  errors,
  modelCatalogError,
  modelCatalogLoading,
  modelCatalogRefreshing,
  onDraftChange,
  onRefreshCatalog,
  onOpenProviderSettings,
  providerOptions,
  providersLoading,
  runtimeModels,
  className,
  ...props
}: AgentCreateRuntimeFieldsProps) {
  const runtimeValue: RuntimeSelectorValue = {
    provider: draft.provider,
    model: draft.model,
    reasoning_effort: draft.reasoningEffort,
    ...(draft.acpOptions ? { acp_options: draft.acpOptions } : {}),
  };
  const hasRuntimeOverride = hasAgentRuntimeOverride(draft);
  return (
    <div
      className={cn("grid min-w-0 gap-4.5", className)}
      data-testid="agent-create-runtime"
      {...props}
    >
      <Field data-invalid={Boolean(errors.provider || errors.reasoningEffort)}>
        <FieldHeader className="w-full">
          <FieldLabel htmlFor="agent-create-runtime-trigger" id="agent-create-runtime-label">
            Model
          </FieldLabel>
          <HelpTip label="About model">
            Provider, model, Reasoning, Fast, and advanced options come from the live catalog. Leave
            them unchanged to use the project defaults.
          </HelpTip>
          {hasRuntimeOverride ? (
            <Button
              className="ml-auto -my-1"
              data-testid="agent-create-runtime-use-project-defaults"
              onClick={() =>
                onDraftChange({
                  ...draft,
                  provider: "",
                  model: "",
                  reasoningEffort: "",
                  speed: "",
                  acpOptions: undefined,
                })
              }
              size="sm"
              type="button"
              variant="link"
            >
              Use project defaults
            </Button>
          ) : null}
        </FieldHeader>
        {draft.provider.trim().length === 0 ? (
          <FieldDescription data-testid="agent-create-runtime-inherited">
            The project's default model will be used.
          </FieldDescription>
        ) : null}
        <RuntimeSelector
          ariaLabelledby="agent-create-runtime-label"
          disabled={providersLoading || providerOptions.length === 0}
          loading={modelCatalogLoading}
          models={runtimeModels}
          onChange={(next, normalizedSpeed) =>
            onDraftChange({
              ...draft,
              provider: next.provider,
              model: next.model,
              reasoningEffort: next.reasoning_effort,
              acpOptions: next.acp_options,
              ...(normalizedSpeed ? { speed: normalizedSpeed } : {}),
            })
          }
          onOpenProviderSettings={onOpenProviderSettings}
          onRefreshCatalog={onRefreshCatalog}
          providers={providerOptions}
          refreshing={modelCatalogRefreshing}
          speed={draft.speed || "normal"}
          onSpeedChange={speed => onDraftChange({ ...draft, speed })}
          triggerId="agent-create-runtime-trigger"
          triggerTestId="agent-create-runtime-select"
          value={runtimeValue}
        />
        <FieldError data-testid="agent-create-provider-error">
          {errors.provider ?? errors.reasoningEffort}
        </FieldError>
        {modelCatalogError ? (
          <p className="text-form-hint text-warning" data-testid="agent-create-model-error">
            {modelCatalogError}
          </p>
        ) : null}
      </Field>
    </div>
  );
}
