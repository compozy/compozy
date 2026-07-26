import { Button, Pill, Spinner } from "@agh/ui";

import type { WindowManagerLayoutEditorModel } from "../../hooks/use-window-manager-layout-editor";

interface LayoutReviewBarProps {
  editor: WindowManagerLayoutEditorModel;
}

type ReviewTone = "neutral" | "warning" | "success" | "danger";

/**
 * The gate between an edit and the workspace. It is docked inside the stage
 * rather than floating: the layout document and the global settings are two
 * different saves, and two overlapping floating bars is how the old page read.
 *
 * Apply stays shut until the daemon has validated *this* draft — the review is
 * keyed by a fingerprint of the document, so a single keystroke afterwards
 * disarms it again.
 */
export function LayoutReviewBar({ editor }: LayoutReviewBarProps) {
  const reviewing = editor.review.isFetching;
  const applying = editor.apply.isPending;
  const diagnostics = editor.validation?.valid === false ? editor.validation.diagnostics : [];
  const status = describe(editor, { reviewing, applying, problems: diagnostics.length });
  const error = editor.importError ?? errorMessage(editor.mutationError);

  return (
    <div className="border-t border-line bg-canvas-tint" data-testid="layout-review-bar">
      {diagnostics.length > 0 ? (
        <div className="flex flex-col gap-1 bg-danger-tint px-3.5 py-2.5" role="status">
          <p className="text-form-label text-danger">The daemon refused this layout</p>
          {diagnostics.map(diagnostic => (
            <p
              className="font-mono text-form-hint leading-normal text-muted"
              key={`${diagnostic.code}:${diagnostic.path ?? ""}`}
            >
              <span className="text-danger">{diagnostic.code}</span>{" "}
              {diagnostic.path ? `${diagnostic.path} — ` : ""}
              {diagnostic.message}
            </p>
          ))}
        </div>
      ) : null}

      <div className="flex flex-wrap items-center gap-2.5 px-3.5 py-2.5">
        <span
          aria-live="polite"
          className="flex min-w-0 items-center gap-2 text-small-body text-muted"
          data-state={status.tone}
          data-testid="layout-review-status"
        >
          {reviewing || applying ? (
            <Spinner className="size-3 shrink-0 text-warning" />
          ) : (
            <Pill.Dot tone={status.tone} />
          )}
          <span className="truncate">{status.message}</span>
        </span>
        <span className="flex-1" />
        {error ? (
          <span className="truncate text-form-hint text-danger" role="alert">
            {error}
          </span>
        ) : null}
        <Button
          data-testid="layout-review-reset"
          disabled={!editor.dirty || applying}
          size="sm"
          type="button"
          variant="ghost"
          onClick={editor.reset}
        >
          Discard
        </Button>
        <Button
          data-testid="layout-review-check"
          disabled={!editor.dirty || reviewing || applying}
          size="sm"
          type="button"
          variant="outline"
          onClick={() => void editor.review.refetch()}
        >
          Review changes
        </Button>
        <Button
          data-testid="layout-review-apply"
          disabled={!editor.reviewCurrent || applying}
          size="sm"
          type="button"
          onClick={() => editor.apply.mutate()}
        >
          Apply layout
        </Button>
      </div>
    </div>
  );
}

function describe(
  editor: WindowManagerLayoutEditorModel,
  state: { reviewing: boolean; applying: boolean; problems: number }
): { message: string; tone: ReviewTone } {
  if (state.applying) return { message: "Applying…", tone: "success" };
  if (state.reviewing) return { message: "Checking with the daemon…", tone: "warning" };
  if (!editor.dirty) {
    return { message: `Revision ${editor.revision} · no unsaved layout edits`, tone: "neutral" };
  }
  if (state.problems > 0) {
    return {
      message: `${state.problems} problem${state.problems === 1 ? "" : "s"} to fix before this can apply`,
      tone: "danger",
    };
  }
  const preview = editor.reviewCurrent ? editor.reviewed?.preview : null;
  if (preview) {
    if (!preview.changed) {
      return {
        message: `Checked · this layout matches revision ${editor.revision}`,
        tone: "success",
      };
    }
    return { message: `Checked · ${changeSummary(preview.changes)}`, tone: "success" };
  }
  return {
    message: "Unreviewed edits · the daemon checks them before they apply",
    tone: "warning",
  };
}

function changeSummary(changes: {
  desktopIds: string[];
  groupIds: string[];
  nodeIds: string[];
  windowIds: string[];
}): string {
  const parts = [
    plural(changes.desktopIds.length, "desktop"),
    plural(changes.groupIds.length, "group"),
    plural(changes.nodeIds.length, "branch"),
    plural(changes.windowIds.length, "window"),
  ].filter((part): part is string => part !== null);
  return parts.length === 0 ? "nothing changes" : `changes ${parts.join(", ")}`;
}

function plural(count: number, noun: string): string | null {
  if (count === 0) return null;
  return `${count} ${noun}${count === 1 ? "" : "s"}`;
}

function errorMessage(error: unknown): string | null {
  return error instanceof Error ? error.message : null;
}
