import { RuntimeSelector } from "@/systems/runtime";

import { useSessionPromptRuntimeContext } from "../hooks/use-session-prompt-runtime-context";

export interface SessionPromptRuntimeSelectorProps {
  canPrompt: boolean;
}

/** Runtime control in the session composer; it configures the next prompt only. */
export function SessionPromptRuntimeSelector({ canPrompt }: SessionPromptRuntimeSelectorProps) {
  const runtime = useSessionPromptRuntimeContext();
  const disabled = !canPrompt || !runtime.canSelectRuntime;

  return (
    <div className="flex min-w-0 items-center gap-1">
      <span id="session-prompt-runtime-label" className="text-form-hint text-faint">
        Next prompt
      </span>
      <RuntimeSelector
        ariaLabelledby="session-prompt-runtime-label"
        catalogStatus={
          runtime.catalog.error || runtime.catalog.refreshError ? (
            <p className="px-3 py-2 text-form-hint text-danger" role="status">
              {runtime.catalog.refreshError ?? runtime.catalog.error}
            </p>
          ) : runtime.catalog.stale ? (
            <p className="px-3 py-2 text-form-hint text-warning" role="status">
              Model catalog may be out of date.
            </p>
          ) : undefined
        }
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
    </div>
  );
}
