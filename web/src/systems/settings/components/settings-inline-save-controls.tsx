import { Button, Spinner } from "@compozy/ui";

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

interface InlineSaveState {
  isDirty: boolean;
  isSaving: boolean;
  error: string | null;
  warnings: string[];
  lastAppliedLabel?: string | null;
}

/** The one status line beside the controls: error, then warnings, then progress. */
function inlineSaveMessage({
  error,
  warnings,
  isSaving,
  isDirty,
  lastAppliedLabel,
}: InlineSaveState): string | null | undefined {
  if (error) return error;
  if (warnings.length > 0) return lastAppliedLabel ?? "Saved with warnings";
  if (isSaving) return "Saving…";
  if (isDirty) return "Unsaved changes";
  return lastAppliedLabel;
}

function inlineSaveMessageClass(error: string | null, warnings: string[]): string {
  if (error) return "text-eyebrow text-danger";
  if (warnings.length > 0) return "text-eyebrow text-warning";
  return "text-eyebrow text-muted";
}

function InlineSaveMessage({ testIdPrefix, ...state }: InlineSaveState & { testIdPrefix: string }) {
  const message = inlineSaveMessage(state);
  if (!message) return null;
  return (
    <span
      aria-live={state.error ? "assertive" : "polite"}
      className={inlineSaveMessageClass(state.error, state.warnings)}
      data-testid={`${testIdPrefix}-message`}
    >
      {message}
      {state.warnings.length > 0 ? ` · ${state.warnings.join(" · ")}` : ""}
    </span>
  );
}

function InlineSaveActions({
  testIdPrefix,
  saveLabel,
  isDirty,
  isSaving,
  canSave,
  onSave,
  onReset,
}: {
  testIdPrefix: string;
  saveLabel: string;
  isDirty: boolean;
  isSaving: boolean;
  canSave: boolean;
  onSave: () => void;
  onReset: () => void;
}) {
  const idle = isDirty && !isSaving;
  return (
    <>
      <Button
        data-testid={`${testIdPrefix}-reset`}
        disabled={!idle}
        onClick={onReset}
        size="sm"
        type="button"
        variant="ghost"
      >
        Discard
      </Button>
      <Button
        data-testid={`${testIdPrefix}-save`}
        disabled={!idle || !canSave}
        onClick={onSave}
        size="sm"
        type="button"
      >
        {isSaving ? (
          <>
            <Spinner className="size-3.5" />
            Saving…
          </>
        ) : (
          saveLabel
        )}
      </Button>
    </>
  );
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
  return (
    <div className="flex flex-wrap items-center gap-2" data-dirty={isDirty} data-testid={testId}>
      <InlineSaveMessage
        error={error}
        isDirty={isDirty}
        isSaving={isSaving}
        lastAppliedLabel={lastAppliedLabel}
        testIdPrefix={controlTestIdPrefix}
        warnings={warnings}
      />
      <InlineSaveActions
        canSave={canSave}
        isDirty={isDirty}
        isSaving={isSaving}
        onReset={onReset}
        onSave={onSave}
        saveLabel={saveLabel}
        testIdPrefix={controlTestIdPrefix}
      />
    </div>
  );
}
