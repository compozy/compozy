import { CheckCircle2, FlaskConical, Info, Play } from "@compozy/ui/icons";

import { Button, Spinner } from "@compozy/ui";

import { ProfileDestinationChip } from "@/systems/profiles";

export function LoopRunFormActions({
  busy,
  note,
  onDryRun,
  onRun,
  pendingKind,
  profileDestination,
  valid,
}: {
  busy: boolean;
  note: { kind: "plan" | "missing" | "idle"; text: string };
  onDryRun: () => void;
  onRun: () => void;
  pendingKind: "dry-run" | "run" | null;
  /** Where the run will be filed, stated only while the aggregate is on. */
  profileDestination: string | null;
  valid: boolean;
}) {
  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-line-soft pt-4">
      <span
        className={
          note.kind === "plan"
            ? "flex flex-1 items-center gap-1.5 text-form-hint text-success"
            : "flex flex-1 items-start gap-1.5 text-form-hint leading-relaxed text-subtle"
        }
      >
        {note.kind === "plan" ? (
          <CheckCircle2 className="size-3.5 shrink-0" aria-hidden="true" />
        ) : null}
        {note.kind === "idle" ? (
          <Info className="mt-0.5 size-3.5 shrink-0 text-faint" aria-hidden="true" />
        ) : null}
        {note.text}
      </span>
      {profileDestination ? <ProfileDestinationChip profile={profileDestination} /> : null}
      <Button
        type="button"
        variant="secondary"
        size="sm"
        data-testid="loop-run-dry-button"
        disabled={busy}
        onClick={onDryRun}
      >
        {pendingKind === "dry-run" ? (
          <Spinner className="size-3.5" />
        ) : (
          <FlaskConical className="size-3.5" aria-hidden="true" />
        )}
        Dry run
      </Button>
      <Button
        type="button"
        variant="primary"
        size="sm"
        data-testid="loop-run-submit-button"
        disabled={busy || !valid}
        onClick={onRun}
      >
        {pendingKind === "run" ? (
          <Spinner className="size-3.5" />
        ) : (
          <Play className="size-3.5" aria-hidden="true" />
        )}
        Start run
      </Button>
    </div>
  );
}
