import { RuntimeSelector } from "@/systems/runtime";
import { Button } from "@compozy/ui";

import { useSessionPromptRuntimeContext } from "../hooks/use-session-prompt-runtime-context";
import { useSessionPromptRuntime } from "../hooks/use-session-prompt-runtime";

export interface SessionPromptRuntimeSelectorProps {
  canPrompt: boolean;
}

/** Runtime control in the session composer; it configures the next prompt only. */
export function SessionPromptRuntimeSelector({ canPrompt }: SessionPromptRuntimeSelectorProps) {
  const runtimeStore = useSessionPromptRuntimeContext();
  const runtime = useSessionPromptRuntime(runtimeStore);
  const disabled = !canPrompt || !runtime.canSelectRuntime;
  const catalogStatus =
    runtime.catalog.error || runtime.catalog.refreshError ? (
      <p className="px-3 py-2 text-form-hint text-danger" role="status">
        {runtime.catalog.refreshError ?? runtime.catalog.error}
      </p>
    ) : runtime.catalog.stale ? (
      <p className="px-3 py-2 text-form-hint text-warning" role="status">
        Model catalog may be out of date.
      </p>
    ) : undefined;

  return (
    <div className="flex min-w-0 items-center gap-1">
      <span id="session-prompt-runtime-label" className="sr-only">
        Runtime for next prompt
      </span>
      <RuntimeSelector
        ariaLabelledby="session-prompt-runtime-label"
        catalogStatus={catalogStatus}
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
      {runtime.availabilityError ? (
        <div className="flex min-w-0 items-center gap-1.5" role="status">
          <p className="truncate text-form-hint text-danger">{runtime.availabilityError}</p>
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
