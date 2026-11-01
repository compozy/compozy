import { Button } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { useSessionPromptRuntimeContext } from "../hooks/use-session-prompt-runtime-context";
import { useSessionPromptRuntime } from "../hooks/use-session-prompt-runtime";
import { RuntimeSelector } from "@/systems/runtime";

export interface SessionPromptRuntimeSelectorProps {
  canPrompt: boolean;
}

/**
 * Runtime control in the session composer. A saved change reaches a live agent when
 * its provider accepts session config changes; the note says when it takes effect.
 */
export function SessionPromptRuntimeSelector({ canPrompt }: SessionPromptRuntimeSelectorProps) {
  const runtimeStore = useSessionPromptRuntimeContext();
  const runtime = useSessionPromptRuntime(runtimeStore);
  const disabled = !canPrompt || !runtime.canSelectRuntime;
  const catalogStatus =
    runtime.catalog.refreshError ||
    runtime.catalog.error ||
    (runtime.catalog.stale && Object.keys(runtime.catalog.providerIssues).length === 0
      ? "Model catalog may be out of date."
      : null);

  return (
    <div className="flex min-w-0 items-center gap-1">
      <span id="session-prompt-runtime-label" className="sr-only">
        Agent and model for this session
      </span>
      <RuntimeSelector
        ariaLabelledby="session-prompt-runtime-label"
        catalogStatus={catalogStatus}
        providerCatalogIssues={runtime.catalog.providerIssues}
        disabled={disabled}
        loading={runtime.catalog.loading}
        models={runtime.catalog.models}
        onChange={runtime.onRuntimeChange}
        onRefreshCatalog={runtime.catalog.refresh}
        onSpeedChange={runtime.onSpeedChange}
        providers={runtime.catalog.providers}
        refreshing={runtime.catalog.refreshing}
        speed={runtime.speed}
        triggerId="session-prompt-runtime"
        triggerTestId="session-prompt-runtime-select"
        value={runtime.value}
        variant="composer"
      />
      {runtime.applyNotice && !runtime.availabilityError ? (
        <p
          role="status"
          data-testid="session-prompt-runtime-apply-notice"
          className={cn(
            "truncate text-form-hint",
            runtime.applyNotice.tone === "destructive" ? "text-destructive" : "text-subtle"
          )}
        >
          {runtime.applyNotice.text}
        </p>
      ) : null}
      {runtime.availabilityError ? (
        <div className="flex min-w-0 items-center gap-1.5" role="status">
          <p className="truncate text-form-hint text-destructive">{runtime.availabilityError}</p>
          <Button
            type="button"
            variant="ghost"
            size="xs"
            className="shrink-0"
            onClick={runtime.onRetryProviders}
          >
            Retry
          </Button>
        </div>
      ) : null}
    </div>
  );
}
