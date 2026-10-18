import { Check, Upload } from "lucide-react";

import { Button, Pill } from "@compozy/ui";

import { loopSourceLabel } from "../../lib/loop-catalog";
import type { LoopSource } from "../../types";

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
  source?: LoopSource;
}

export function LoopEditorTopbarStatus({
  version,
  isDirty,
  positionsDirty,
  source,
}: LoopEditorTopbarStatusProps) {
  const readOnlySource = source !== undefined && source !== "workspace";
  const state = readOnlySource
    ? loopSourceLabel({ source })
    : isDirty
      ? "Draft"
      : positionsDirty
        ? "Layout not saved"
        : "Published";
  // Only unpublished edits carry signal color; a resting published Loop stays neutral.
  const tone = !readOnlySource && isDirty ? "warning" : "neutral";

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
        form="plain"
        title={`Version ${version ?? "?"} · ${state}`}
      >
        <Pill.Dot tone={tone} />v{version ?? "?"} · {state}
      </Pill>
    </span>
  );
}

/**
 * Trailing shell-topbar actions for the loop editor: a quiet validation action
 * and the publish action. Window heads carry no inverted primary (polish P4).
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
        variant="ghost"
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
        variant="secondary"
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
