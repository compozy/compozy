import { Button, Spinner } from "@agh/ui";

export interface SettingsInlineSaveControlsProps {
  testId: string;
  controlTestIdPrefix?: string;
  saveLabel: string;
  isDirty: boolean;
  isSaving: boolean;
  error: string | null;
  warnings?: string[];
  lastAppliedLabel?: string | null;
  canSave?: boolean;
  onSave: () => void;
  onReset: () => void;
}

export function SettingsInlineSaveControls({
  testId,
  controlTestIdPrefix = testId,
  saveLabel,
  isDirty,
  isSaving,
  error,
  warnings = [],
  lastAppliedLabel,
  canSave = true,
  onSave,
  onReset,
}: SettingsInlineSaveControlsProps) {
  const message = error
    ? error
    : warnings.length > 0
      ? (lastAppliedLabel ?? "Saved with warnings")
      : isSaving
        ? "Saving…"
        : isDirty
          ? "Unsaved changes"
          : lastAppliedLabel;
  return (
    <div className="flex flex-wrap items-center gap-2" data-dirty={isDirty} data-testid={testId}>
      {message ? (
        <span
          aria-live={error ? "assertive" : "polite"}
          className={
            error
              ? "text-xs text-danger"
              : warnings.length
                ? "text-xs text-warning"
                : "text-xs text-muted"
          }
          data-testid={`${controlTestIdPrefix}-message`}
        >
          {message}
          {warnings.length > 0 ? ` · ${warnings.join(" · ")}` : ""}
        </span>
      ) : null}
      <Button
        data-testid={`${controlTestIdPrefix}-reset`}
        disabled={!isDirty || isSaving}
        onClick={onReset}
        size="sm"
        type="button"
        variant="ghost"
      >
        Discard
      </Button>
      <Button
        data-testid={`${controlTestIdPrefix}-save`}
        disabled={!isDirty || isSaving || !canSave}
        onClick={onSave}
        size="sm"
        type="button"
      >
        {isSaving ? <Spinner className="size-3" /> : null}
        {isSaving ? "Saving…" : saveLabel}
      </Button>
    </div>
  );
}
