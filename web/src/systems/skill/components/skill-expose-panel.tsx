import { Button, cn, MonoId, StatusDot } from "@compozy/ui";

import type { SkillExposeModel } from "../hooks/use-skill-expose";
import type { SkillExposeResultView, SkillExposureView } from "../lib/skill-exposure-view";
import type { SkillExposeTarget } from "../types";
import { SkillExposeTargetPicker } from "./skill-expose-target-picker";

interface SkillExposePanelProps {
  exposures: readonly SkillExposureView[];
  targets: readonly SkillExposeTarget[];
  targetsLoading?: boolean;
  targetsError?: string | null;
  onRetryTargets?: () => void;
  model: SkillExposeModel;
  /** Label for each target, so a pending row reads like the row it becomes. */
  labelForTarget: (slug: string) => string;
}

const TEST_ID = "skill-expose-panel";

/**
 * Which other tools can see this skill, and whether each link still works.
 *
 * Statuses come from the daemon's reconcile against the filesystem, so a link
 * we created and can repair reads differently from a file someone else put at
 * the same path — and the second one gets no action at all.
 */
export function SkillExposePanel({
  exposures,
  targets,
  targetsLoading = false,
  targetsError = null,
  onRetryTargets,
  model,
  labelForTarget,
}: SkillExposePanelProps) {
  const pendingTargets = new Set(model.pendingTargets);
  const listed = new Set(exposures.map(exposure => exposure.target));
  const pending = model.pendingTargets.filter(target => !listed.has(target));

  return (
    <div className="flex flex-col gap-2.5 px-3.5 py-1.5" data-testid={TEST_ID}>
      {model.failure !== null ? (
        <div
          className="flex flex-col gap-1.5 rounded-md border border-danger-line bg-danger-tint px-3 py-2"
          data-testid={`${TEST_ID}-failure`}
          role="alert"
        >
          <p className="text-small-body text-danger">
            <b>Couldn&rsquo;t update exposures.</b> {model.failure}
            {model.rolledBack ? " The target that had finished was undone." : ""}
          </p>
          <div>
            <Button onClick={model.dismiss} size="sm" type="button" variant="ghost">
              Dismiss
            </Button>
          </div>
        </div>
      ) : null}

      {exposures.length === 0 && pending.length === 0 ? null : (
        <div className="flex flex-col" data-testid={`${TEST_ID}-list`}>
          {exposures.map(exposure => (
            <ExposureRow
              busy={model.isPending}
              exposure={exposure}
              key={exposure.target}
              label={labelForTarget(exposure.target)}
              onExposeAgain={() => model.expose([exposure.target])}
              onUnexpose={() => model.unexpose([exposure.target])}
              pending={pendingTargets.has(exposure.target)}
              pendingAction={model.pendingAction}
            />
          ))}
          {pending.map(target => (
            <PendingRow key={target} label={labelForTarget(target)} action={model.pendingAction} />
          ))}
        </div>
      )}

      {model.results.length > 0 && model.failure !== null ? (
        <ResultLedger results={model.results} labelForTarget={labelForTarget} />
      ) : null}

      <div>
        <SkillExposeTargetPicker
          disabled={model.isPending}
          error={targetsError}
          loading={targetsLoading}
          onExpose={model.expose}
          onRetry={onRetryTargets}
          targets={targets}
        />
      </div>
    </div>
  );
}

function ExposureRow({
  exposure,
  label,
  pending,
  pendingAction,
  busy,
  onExposeAgain,
  onUnexpose,
}: {
  exposure: SkillExposureView;
  label: string;
  pending: boolean;
  pendingAction: "expose" | "unexpose" | null;
  busy: boolean;
  onExposeAgain: () => void;
  onUnexpose: () => void;
}) {
  const testId = `${TEST_ID}-row-${exposure.target}`;
  return (
    <div
      className="flex min-w-0 items-start gap-2 border-b border-line py-2 last:border-b-0"
      data-status={exposure.status}
      data-testid={testId}
    >
      <StatusDot
        className="mt-1 shrink-0"
        label={
          pending ? (pendingAction === "unexpose" ? "removing…" : "exposing…") : exposure.sentence
        }
        tone={pending ? "accent" : exposure.tone}
        variant={pending || exposure.status === "foreign_conflict" ? "ring" : "solid"}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex min-w-0 flex-wrap items-baseline gap-1.5">
          <b className="text-small-body text-fg">{label}</b>
          <span
            className={cn(
              "text-form-hint",
              pending ? "text-subtle" : exposure.repairable ? "text-danger" : "text-subtle"
            )}
            data-testid={`${testId}-status`}
          >
            {pending
              ? pendingAction === "unexpose"
                ? "removing…"
                : "exposing…"
              : exposure.sentence}
          </span>
        </span>
        <span
          className={cn(
            "truncate font-mono text-badge text-faint",
            exposure.stale ? "line-through" : null
          )}
          title={exposure.path}
        >
          {exposure.path}
        </span>
      </div>
      {/* A foreign entry is reported, never touched: no repair, no removal. */}
      {pending || exposure.status === "foreign_conflict" ? null : (
        <span className="flex shrink-0 items-center gap-1">
          {exposure.repairable ? (
            <Button
              data-testid={`${testId}-expose-again`}
              disabled={busy}
              onClick={onExposeAgain}
              size="sm"
              type="button"
              variant="ghost"
            >
              Expose again
            </Button>
          ) : null}
          {exposure.removable ? (
            <Button
              data-testid={`${testId}-unexpose`}
              disabled={busy}
              onClick={onUnexpose}
              size="sm"
              type="button"
              variant="ghost"
            >
              Unexpose
            </Button>
          ) : null}
        </span>
      )}
    </div>
  );
}

/** A target being written has no reconciled state yet, so it claims no status word. */
function PendingRow({ label, action }: { label: string; action: "expose" | "unexpose" | null }) {
  const status = action === "unexpose" ? "removing…" : "exposing…";
  return (
    <div
      className="flex min-w-0 items-center gap-2 border-b border-line py-2 last:border-b-0"
      data-testid={`${TEST_ID}-pending-${label}`}
    >
      <StatusDot label={status} tone="accent" variant="ring" />
      <b className="text-small-body text-subtle">{label}</b>
      <span className="text-form-hint text-subtle">{status}</span>
    </div>
  );
}

function ResultLedger({
  results,
  labelForTarget,
}: {
  results: readonly SkillExposeResultView[];
  labelForTarget: (slug: string) => string;
}) {
  return (
    <div className="flex flex-col gap-1" data-testid={`${TEST_ID}-results`}>
      {results.map(result => (
        <div
          className="flex min-w-0 flex-wrap items-baseline gap-1.5 text-form-hint"
          data-ok={result.ok}
          data-rolled-back={result.rolledBack}
          data-testid={`${TEST_ID}-result-${result.target}`}
          key={result.target}
        >
          <span className="text-fg">{labelForTarget(result.target)}</span>
          <span className={result.ok ? "text-subtle" : "text-danger"}>{result.sentence}</span>
          {result.code !== null ? (
            <MonoId className="text-faint" preserveCase value={result.code} />
          ) : null}
        </div>
      ))}
    </div>
  );
}
