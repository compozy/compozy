import { Check, Upload } from "lucide-react";

import { Button, Pill } from "@agh/ui";

interface LoopEditorTopbarActionsProps {
  busy: boolean;
  publishDisabled: boolean;
  onValidate: () => void;
  onPublish: () => void;
}

interface LoopEditorTopbarStatusProps {
  version: number | undefined;
  isDirty: boolean;
  positionsDirty: boolean;
}

export function LoopEditorTopbarStatus({
  version,
  isDirty,
  positionsDirty,
}: LoopEditorTopbarStatusProps) {
  const state = isDirty ? "unpublished edits" : positionsDirty ? "layout unsaved" : "published";
  const tone = isDirty ? "warning" : positionsDirty ? "neutral" : "success";

  return (
    <span
      data-testid={
        isDirty
          ? "loop-editor-dirty-chip"
          : positionsDirty
            ? "loop-editor-layout-dirty-chip"
            : undefined
      }
    >
      <Pill
        data-testid="loop-editor-version"
        mono
        size="xs"
        title={`v${version ?? "?"} · ${state}`}
        tone={tone}
      >
        <Pill.Dot size="sm" tone={tone} />v{version ?? "?"} · {state}
      </Pill>
    </span>
  );
}

/**
 * Trailing shell-topbar actions for the loop editor: one secondary validation
 * action and the primary publish action.
 */
export function LoopEditorTopbarActions({
  busy,
  publishDisabled,
  onValidate,
  onPublish,
}: LoopEditorTopbarActionsProps) {
  return (
    <div className="flex items-center gap-2" data-testid="loop-editor-topbar-actions">
      <Button
        type="button"
        variant="neutral"
        size="sm"
        disabled={busy}
        onClick={onValidate}
        data-testid="loop-editor-validate"
      >
        <Check aria-hidden="true" className="size-3.5" />
        Validate
      </Button>
      <Button
        type="button"
        size="sm"
        disabled={publishDisabled}
        onClick={onPublish}
        data-testid="loop-editor-publish"
      >
        <Upload aria-hidden="true" className="size-3.5" />
        Publish
      </Button>
    </div>
  );
}
