/** The Model row of the setup list: the runtime selector itself, so a pick applies at once. */
import { useId } from "react";
import { Bot } from "@compozy/ui/icons";

import { Button, cn } from "@compozy/ui";

import type { UseAgentRuntimeEditorResult } from "../../hooks/use-agent-runtime-editor";
import { RuntimeSelector } from "@/systems/runtime";
import {
  HOME_RUN_ROW_CLASS,
  HOME_RUN_SUBTITLE_CLASS,
  HOME_RUN_TITLE_CLASS,
} from "@/systems/dashboard";

export function AgentSetupModelRow({
  model,
  readOnly,
}: {
  model: UseAgentRuntimeEditorResult;
  readOnly: boolean;
}) {
  const labelId = useId();
  const triggerId = useId();
  const problem = model.conflictMessage ?? model.error ?? model.providerSourceError;
  // A failed conflict reread or provider load leaves nothing to pick from; both can retry.
  const retry =
    model.onReload ??
    (problem !== null && problem === model.providerSourceError
      ? model.onRetryProviderSource
      : null);
  return (
    <div className="px-4 py-3" data-testid="agent-setup-model">
      <div className={cn(HOME_RUN_ROW_CLASS, "p-0")}>
        <span className="min-w-0">
          <span className={HOME_RUN_TITLE_CLASS}>
            <Bot aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
            <span className="truncate" id={labelId}>
              Model
            </span>
          </span>
          {model.overridden ? null : (
            <span className={HOME_RUN_SUBTITLE_CLASS}>
              <span className="truncate pl-6">Uses your default</span>
            </span>
          )}
        </span>
        {model.overridden && !readOnly ? (
          <Button
            data-testid="agent-setup-model-use-default"
            disabled={model.isPending}
            onClick={model.onUseDefault}
            size="xs"
            type="button"
            variant="link"
          >
            Use default
          </Button>
        ) : null}
      </div>
      <div className="mt-2 pl-6">
        <RuntimeSelector
          ariaLabelledby={labelId}
          disabled={model.isPending || model.providersLoading || model.providerOptions.length === 0}
          loading={model.modelCatalogLoading}
          models={model.runtimeModels}
          onChange={model.onChange}
          onOpenProviderSettings={model.onOpenProviderSettings}
          onRefreshCatalog={model.onRefreshCatalog}
          onSpeedChange={model.onSpeedChange}
          providers={model.providerOptions}
          readOnly={readOnly}
          refreshing={model.modelCatalogRefreshing}
          speed={model.speed}
          triggerId={triggerId}
          triggerTestId="agent-setup-model-select"
          value={model.value}
          variant="small"
          {...(model.modelCatalogError ? { catalogStatus: model.modelCatalogError } : {})}
        />
      </div>
      {problem ? (
        <div className="mt-2 flex min-w-0 items-center gap-1.5 pl-6" role="alert">
          <p className="text-small-body text-destructive" data-testid="agent-setup-model-error">
            {problem}
          </p>
          {retry ? (
            <Button className="shrink-0" onClick={retry} size="xs" type="button" variant="ghost">
              Retry
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
