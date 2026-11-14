import { useState, type ReactNode } from "react";
import { Info, RotateCcw, Trash2 } from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  DateTimePicker,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  Field,
  FieldDescription,
  FieldLabel,
  FieldLegend,
  FieldSet,
  PillGroup,
  Progress,
  ProgressLabel,
  ProgressValue,
  Spinner,
} from "@compozy/ui";

import {
  CLEANUP_RANGE_ITEMS,
  DEFAULT_CLEANUP_OPTIONS,
  TYPED_CONFIRM_THRESHOLD,
  cleanupActionLabel,
  cleanupChoiceCounts,
  cleanupDetail,
  cleanupHeadline,
  cleanupImpact,
  emptyReason,
  formatCleanupDay,
  rangeKeeps,
  plural,
  type ArchivedThread,
  type CleanupImpact,
  type CleanupOptions,
  typedMatches,
} from "./archived-cleanup-model";
import { CountBadge, TypedConfirm, WhatChecks } from "./archived-cleanup-fields";

/** Where the run is: choosing, deleting, or how a failed run ended. Success closes the dialog. */
export type CleanupPhase =
  | { kind: "form" }
  | { kind: "deleting"; done: number }
  | {
      kind: "partial";
      /** Main sessions the run set out to delete, and how many went. */
      total: number;
      deleted: number;
      failed: readonly { title: string; reason: string }[];
    }
  | { kind: "failed"; reason: string };

export interface ArchivedCleanupDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The list's scope, as the sidebar names it: a project, or "All projects". */
  scopeName: string;
  /** All projects: the preview breaks the count down per project. */
  acrossProjects?: boolean;
  /** The archive in the list's scope; the implementation reads the dry run instead. */
  archive: readonly ArchivedThread[];
  initialOptions?: Partial<CleanupOptions>;
  phase: CleanupPhase;
  onConfirm: (options: CleanupOptions) => void;
  onRetry?: () => void;
  /** Stops a running cleanup after the sessions in flight; what went stays gone. */
  onStop?: () => void;
}

/** Per project, busiest first; the long tail folds into one phrase. */
function ProjectBreakdown({ impact }: { impact: CleanupImpact }) {
  const shown = impact.projects.slice(0, 4);
  const rest = impact.projects.slice(4);
  const restCount = rest.reduce((sum, item) => sum + item.sessions, 0);
  return (
    <ul className="mt-1.5 flex flex-wrap gap-x-3 gap-y-1 text-meta">
      {shown.map(item => (
        <li className="flex items-baseline gap-1" key={item.project}>
          <span className="text-fg-2">{item.project}</span>
          <CountBadge count={item.sessions} />
        </li>
      ))}
      {rest.length > 0 ? (
        <li className="flex items-baseline gap-1">
          <span className="text-fg-2">{plural(rest.length, "more project", "more projects")}</span>
          <CountBadge count={restCount} />
        </li>
      ) : null}
    </ul>
  );
}

/** The live answer to "what happens if I press it", and the one warning the run needs. */
function ImpactPreview({
  options,
  impact,
  acrossProjects,
}: {
  options: CleanupOptions;
  impact: CleanupImpact;
  acrossProjects: boolean;
}) {
  if (impact.total === 0) {
    return (
      <Alert data-testid="archived-cleanup-preview" role="status">
        <Info />
        <AlertTitle>Nothing to delete</AlertTitle>
        <AlertDescription>{emptyReason(options)}</AlertDescription>
      </Alert>
    );
  }
  const kept =
    impact.keptLive > 0
      ? `${plural(impact.keptLive, "child session is", "child sessions are")} still in your lists, so ${impact.keptLive === 1 ? "it stays" : "they stay"}.`
      : "";
  return (
    <Alert data-testid="archived-cleanup-preview" role="status" variant="destructive">
      <Trash2 />
      <AlertTitle className="tabular-nums">{cleanupHeadline(impact)}</AlertTitle>
      <AlertDescription className="tabular-nums">
        {cleanupDetail(options.what, impact, kept)}
        {acrossProjects && impact.projects.length > 1 ? <ProjectBreakdown impact={impact} /> : null}
      </AlertDescription>
    </Alert>
  );
}

function DeletingProgress({ done, total }: { done: number; total: number }) {
  return (
    <Progress
      aria-label="Deleting sessions"
      className="rounded-lg bg-surface-2 px-3.5 py-3"
      data-testid="archived-cleanup-progress"
      max={total}
      value={done}
    >
      <ProgressLabel>Deleting sessions…</ProgressLabel>
      <ProgressValue>{() => `${done.toLocaleString()} of ${total.toLocaleString()}`}</ProgressValue>
    </Progress>
  );
}

function CleanupForm({
  archive,
  scopeName,
  acrossProjects,
  initialOptions,
  phase,
  onCancel,
  onConfirm,
  onStop,
}: {
  archive: readonly ArchivedThread[];
  scopeName: string;
  acrossProjects: boolean;
  initialOptions?: Partial<CleanupOptions>;
  phase: Extract<CleanupPhase, { kind: "form" | "deleting" }>;
  onCancel: () => void;
  onConfirm: (options: CleanupOptions) => void;
  onStop?: () => void;
}) {
  const [options, setOptions] = useState<CleanupOptions>(() => ({
    ...DEFAULT_CLEANUP_OPTIONS,
    ...initialOptions,
  }));
  const [typed, setTyped] = useState("");
  const deleting = phase.kind === "deleting";
  const impact = cleanupImpact(archive, options);
  const counts = cleanupChoiceCounts(archive, options);
  const needsTyping = impact.total > TYPED_CONFIRM_THRESHOLD;
  const ready = impact.total > 0 && (!needsTyping || typedMatches(typed, impact.total));
  const update = (next: Partial<CleanupOptions>) =>
    setOptions(current => ({ ...current, ...next }));

  return (
    <form
      className="flex flex-col gap-4"
      noValidate
      onSubmit={event => {
        event.preventDefault();
        if (!deleting && ready) onConfirm(options);
      }}
    >
      <DialogHeader>
        <DialogTitle>Clean up history</DialogTitle>
        <DialogDescription>
          Permanently deletes archived sessions in <strong className="text-fg">{scopeName}</strong>,
          with their conversations. Sessions in your lists and in Later are never touched.
        </DialogDescription>
      </DialogHeader>
      {/* A fieldset, not a Field: a Field fades every checkbox in it once one of them is locked. */}
      <FieldSet className="gap-0">
        <FieldLegend className="mb-2" id="archived-cleanup-what-label" variant="label">
          What to delete
        </FieldLegend>
        <WhatChecks
          counts={counts}
          disabled={deleting}
          onChange={next => update({ what: { ...options.what, ...next } })}
          what={options.what}
        />
      </FieldSet>
      <Field>
        <FieldLabel id="archived-cleanup-range-label">Archived</FieldLabel>
        <PillGroup
          aria-labelledby="archived-cleanup-range-label"
          className="flex-wrap"
          data-testid="archived-cleanup-range"
          items={CLEANUP_RANGE_ITEMS.map(item => ({
            ...item,
            disabled: deleting,
            testId: `archived-cleanup-range-${item.value}`,
          }))}
          onChange={range => update({ range })}
          value={options.range}
        />
        {options.range === "before" ? (
          <DateTimePicker
            aria-labelledby="archived-cleanup-range-label"
            className="mt-1"
            data-testid="archived-cleanup-before"
            defaultTime={[0, 0]}
            disabled={deleting}
            formatValue={day => `Before ${formatCleanupDay(day)}`}
            id="archived-cleanup-before"
            onValueChange={before => update({ before })}
            placeholder="Pick a date"
            value={options.before}
          />
        ) : null}
        <FieldDescription>
          {rangeKeeps(options)} Child sessions and subagents follow their main session.
        </FieldDescription>
      </Field>
      {deleting ? (
        <Field>
          <DeletingProgress done={phase.done} total={impact.total} />
          <FieldDescription>It keeps going if you close this.</FieldDescription>
        </Field>
      ) : (
        <>
          <ImpactPreview acrossProjects={acrossProjects} impact={impact} options={options} />
          {needsTyping ? (
            <TypedConfirm onChange={setTyped} total={impact.total} value={typed} />
          ) : null}
        </>
      )}
      <DialogFooter className="gap-2">
        {deleting ? (
          <>
            <Button
              data-testid="archived-cleanup-stop"
              onClick={onStop}
              type="button"
              variant="ghost"
            >
              Stop
            </Button>
            <Button
              data-testid="archived-cleanup-close"
              onClick={onCancel}
              type="button"
              variant="secondary"
            >
              <Spinner className="size-3" />
              Close
            </Button>
          </>
        ) : (
          <>
            <Button
              data-testid="archived-cleanup-cancel"
              onClick={onCancel}
              type="button"
              variant="ghost"
            >
              Cancel
            </Button>
            <Button
              data-testid="archived-cleanup-confirm"
              disabled={!ready}
              type="submit"
              variant="destructive"
            >
              <Trash2 className="size-3" />
              {impact.total === 0 ? "Delete" : cleanupActionLabel(impact)}
            </Button>
          </>
        )}
      </DialogFooter>
    </form>
  );
}

/** The run failed in part or whole: say what happened in one sentence, then offer the retry. */
function CleanupOutcome({
  title,
  description,
  children,
  actions,
}: {
  title: string;
  description: ReactNode;
  children?: ReactNode;
  actions: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-4">
      <DialogHeader>
        <DialogTitle>{title}</DialogTitle>
        <DialogDescription className="tabular-nums" role="status">
          {description}
        </DialogDescription>
      </DialogHeader>
      {children}
      <DialogFooter className="gap-2">{actions}</DialogFooter>
    </div>
  );
}

/**
 * "Clean up history" for the Archived list: what to delete, from when, and a live count of what
 * goes. The count-bearing destructive action confirms; past 100 sessions the person types the
 * count first. The run happens in the daemon, so the dialog can close while it works.
 */
export function ArchivedCleanupDialog({
  open,
  onOpenChange,
  scopeName,
  acrossProjects = false,
  archive,
  initialOptions,
  phase,
  onConfirm,
  onRetry,
  onStop,
}: ArchivedCleanupDialogProps) {
  const close = () => onOpenChange(false);
  const body = (() => {
    switch (phase.kind) {
      case "partial": {
        const failed = phase.failed.length;
        return (
          <CleanupOutcome
            actions={
              <>
                <Button onClick={close} variant="ghost">
                  Close
                </Button>
                <Button onClick={onRetry} variant="destructive">
                  <RotateCcw className="size-3" />
                  Try {failed} again
                </Button>
              </>
            }
            description={`Deleted ${phase.deleted.toLocaleString()} of ${plural(phase.total, "session", "sessions")}. ${plural(failed, "archived session stays", "archived sessions stay")} in Archived with everything under ${failed === 1 ? "it" : "them"}.`}
            title="Some sessions weren’t deleted"
          >
            <ul
              className="flex flex-col gap-1.5 rounded-lg bg-surface-2 px-3.5 py-3"
              data-testid="archived-cleanup-failures"
            >
              {phase.failed.map(item => (
                <li className="flex min-w-0 flex-col gap-0.5" key={item.title}>
                  <span className="truncate text-small-body text-fg">{item.title}</span>
                  <span className="text-meta text-destructive">{item.reason}</span>
                </li>
              ))}
            </ul>
          </CleanupOutcome>
        );
      }
      case "failed":
        return (
          <CleanupOutcome
            actions={
              <>
                <Button onClick={close} variant="ghost">
                  Cancel
                </Button>
                <Button onClick={onRetry} variant="destructive">
                  <RotateCcw className="size-3" />
                  Try again
                </Button>
              </>
            }
            description={`Nothing was deleted. ${phase.reason}`}
            title="Couldn’t clean up history"
          />
        );
      default:
        return (
          <CleanupForm
            acrossProjects={acrossProjects}
            archive={archive}
            initialOptions={initialOptions}
            onCancel={close}
            onConfirm={onConfirm}
            onStop={onStop}
            phase={phase}
            scopeName={scopeName}
          />
        );
    }
  })();
  // A running cleanup belongs to the daemon, so closing the dialog never stops it.
  return (
    <Dialog onOpenChange={onOpenChange} open={open}>
      <DialogContent className="max-w-lg" data-testid="archived-cleanup-dialog">
        {body}
      </DialogContent>
    </Dialog>
  );
}
