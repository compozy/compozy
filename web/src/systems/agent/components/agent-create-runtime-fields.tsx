import type { ComponentProps } from "react";

import { Button, cn, Field, FieldError, FieldHeader, FieldLabel, HelpTip, Input } from "@agh/ui";

import {
  RuntimeSelector,
  type RuntimeModelOption,
  type RuntimeProviderOption,
  type RuntimeSelectorValue,
} from "@/systems/runtime";

import type { AgentCreateDialogDraft } from "../lib/agent-create-draft";

export interface AgentCreateRuntimeFieldsProps extends ComponentProps<"div"> {
  draft: AgentCreateDialogDraft;
  errors: Record<string, string | undefined>;
  modelCatalogError: string | null;
  modelCatalogLoading: boolean;
  modelCatalogLoaded: boolean;
  modelCatalogRefreshing: boolean;
  onDraftChange: (draft: AgentCreateDialogDraft) => void;
  onRefreshCatalog: () => void;
  onOpenProviderSettings: () => void;
  providerOptions: RuntimeProviderOption[];
  providersLoading: boolean;
  runtimeModels: RuntimeModelOption[];
}

/**
 * Simple tier: the runtime selector and the catalog grouping, side by side.
 *
 * Category path lives here rather than under Advanced because it names where
 * the agent files in the catalog — a decision a person makes while naming the
 * agent, not a launch override.
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
  modelCatalogLoaded,
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
  };
  const hasRuntimeOverride = Boolean(
    draft.provider.trim() || draft.model.trim() || draft.reasoningEffort
  );
  return (
    <div
      className={cn("grid min-w-0 gap-4.5 md:grid-cols-2", className)}
      data-testid="agent-create-runtime"
      {...props}
    >
      <Field data-invalid={Boolean(errors.provider || errors.reasoningEffort)}>
        <FieldHeader className="w-full">
          <FieldLabel htmlFor="agent-create-runtime-trigger" id="agent-create-runtime-label">
            Runtime
          </FieldLabel>
          <HelpTip label="About runtime">
            Provider, model, and reasoning effort come from the live catalogs. Leave them unchanged
            to inherit the project defaults; picking any of them creates agent-level overrides.
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
                })
              }
              size="sm"
              type="button"
              variant="ghost"
            >
              Use project defaults
            </Button>
          ) : null}
        </FieldHeader>
        {draft.provider.trim().length === 0 ? (
          <p className="text-form-hint text-info" data-testid="agent-create-runtime-inherited">
            Project runtime defaults will be used.
          </p>
        ) : null}
        <RuntimeSelector
          ariaLabelledby="agent-create-runtime-label"
          catalogLoaded={modelCatalogLoaded}
          disabled={providersLoading || providerOptions.length === 0}
          loading={modelCatalogLoading}
          models={runtimeModels}
          onChange={next =>
            onDraftChange({
              ...draft,
              provider: next.provider,
              model: next.model,
              reasoningEffort: next.reasoning_effort,
            })
          }
          onOpenProviderSettings={onOpenProviderSettings}
          onRefreshCatalog={onRefreshCatalog}
          providers={providerOptions}
          refreshing={modelCatalogRefreshing}
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

      <Field data-invalid={Boolean(errors.categoryPath)}>
        <FieldHeader>
          <FieldLabel htmlFor="agent-create-category-path">Category path</FieldLabel>
          <HelpTip label="About category path">
            Slash-separated catalog grouping, stored as one segment per level. It organises the
            agent catalog and changes nothing about how the agent runs.
          </HelpTip>
        </FieldHeader>
        <Input
          aria-invalid={Boolean(errors.categoryPath)}
          className="font-mono"
          data-testid="agent-create-category-path"
          id="agent-create-category-path"
          onChange={event => onDraftChange({ ...draft, categoryPath: event.target.value })}
          placeholder="operations/incident"
          value={draft.categoryPath}
        />
        <FieldError data-testid="agent-create-category-path-error">
          {errors.categoryPath}
        </FieldError>
      </Field>
    </div>
  );
}
