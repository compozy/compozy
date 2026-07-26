import { AlertCircle } from "lucide-react";

import { Button, cn, Pill, Spinner } from "@agh/ui";

import type { SettingsSaveBarState } from "../lib/save-state";

interface SettingsSaveBarProps {
  slug: string;
  state: SettingsSaveBarState;
  onSave: () => void;
  onReset: () => void;
  className?: string;
}

function warningsFor(state: SettingsSaveBarState): string[] {
  return state.kind === "dirty" || state.kind === "invalid" || state.kind === "warning"
    ? state.warnings
    : [];
}

function messageFor(state: Exclude<SettingsSaveBarState, { kind: "idle" }>): string {
  switch (state.kind) {
    case "dirty":
      return "Unsaved changes";
    case "invalid":
      return "Resolve validation errors before saving";
    case "saving":
      return "Saving…";
    case "error":
    case "saved":
    case "warning":
      return state.message;
  }
}

/** Pure save-state renderer. Mutation and saved-flash orchestration live in route hooks. */
function SettingsSaveBar({ slug, state, onSave, onReset, className }: SettingsSaveBarProps) {
  if (state.kind === "idle") return null;

  const isDirty = state.kind === "dirty" || state.kind === "invalid";
  const isSaving = state.kind === "saving";
  const isError = state.kind === "error";
  const warnings = warningsFor(state);
  const liveRegion = isError ? "assertive" : "polite";
  const dotTone = state.kind === "saved" ? "success" : "warning";

  return (
    <div
      className={cn("pointer-events-none sticky bottom-6 z-10 flex justify-center px-4", className)}
    >
      <div
        aria-atomic="true"
        aria-live={liveRegion}
        className="pointer-events-auto flex w-full max-w-settings-save-bar items-center gap-3 rounded-lg border border-line-strong bg-elevated py-2.5 pr-3 pl-4 shadow-overlay"
        data-dirty={isDirty ? "true" : "false"}
        data-state={state.kind}
        data-testid={`settings-page-${slug}-save-bar`}
        role="status"
      >
        <span className="flex min-w-0 flex-1 items-center gap-2.5 text-small-body text-fg">
          {isSaving ? (
            <Spinner className="size-3 shrink-0 text-warning" />
          ) : isError ? (
            <AlertCircle aria-hidden="true" className="size-3.5 shrink-0 text-danger" />
          ) : (
            <Pill.Dot className="size-settings-save-dot" tone={dotTone} />
          )}
          <span
            className={cn("truncate", isError && "text-danger")}
            data-testid={`settings-page-${slug}-save-message`}
          >
            {messageFor(state)}
          </span>
          {warnings.length > 0 ? (
            <span
              className="truncate text-form-hint text-warning"
              data-testid={`settings-page-${slug}-save-warnings`}
            >
              {warnings.join(" · ")}
            </span>
          ) : null}
        </span>
        <Button
          data-testid={`settings-page-${slug}-reset`}
          disabled={!isDirty || isSaving}
          onClick={onReset}
          size="sm"
          type="button"
          variant="ghost"
        >
          Discard
        </Button>
        <Button
          data-testid={`settings-page-${slug}-save`}
          disabled={state.kind !== "dirty"}
          onClick={onSave}
          size="sm"
          type="button"
        >
          Save changes
        </Button>
      </div>
    </div>
  );
}

export { SettingsSaveBar };
export type { SettingsSaveBarProps };
