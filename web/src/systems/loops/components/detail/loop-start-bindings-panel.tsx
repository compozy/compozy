import { CalendarClock, Plus, Zap } from "lucide-react";

import { Button, Section, Spinner } from "@agh/ui";

import { bindingKindLabel, type LoopBindingRow } from "../../lib/loop-bindings";
import { MonoTag } from "../mono-tag";

interface LoopStartBindingsPanelProps {
  /** The DSL `start[]` allowlist kinds, read-only (edited only in the definition). */
  declaredKinds: readonly string[];
  /** Attached loop-target automations for this Loop (via the `loop=<name>` filter). */
  bindings: readonly LoopBindingRow[];
  jobs?: LoopBindingPagination;
  triggers?: LoopBindingPagination;
  isLoading?: boolean;
  onAddTrigger?: () => void;
  onAddSchedule?: () => void;
}

export interface LoopBindingPagination {
  error?: Error | null;
  hasMore: boolean;
  isFetchingMore: boolean;
  loadMore: () => void;
  loaded: number;
  total: number;
}

const SCHEDULE_KIND = "schedule";
const TRIGGER_KINDS = new Set(["trigger", "webhook"]);

/**
 * The Loop-detail Start-bindings panel (§9.14 / ADR-007): the declared `start[]`
 * kinds as read-only mono chips, one row per attached loop-target automation with
 * an enabled dot (the only stateful color here), and Add CTAs gated to the kinds
 * the Loop's allowlist actually permits — so no CTA leads to a create-time 422.
 */
export function LoopStartBindingsPanel({
  declaredKinds,
  bindings,
  jobs,
  triggers,
  isLoading = false,
  onAddTrigger,
  onAddSchedule,
}: LoopStartBindingsPanelProps) {
  const canAddSchedule = declaredKinds.includes(SCHEDULE_KIND);
  const canAddTrigger = declaredKinds.some(kind => TRIGGER_KINDS.has(kind));
  const hasPagination = Boolean(jobs || triggers);
  const loadedBindings = hasPagination
    ? (jobs?.loaded ?? 0) + (triggers?.loaded ?? 0)
    : bindings.length;
  const totalBindings = hasPagination
    ? (jobs?.total ?? 0) + (triggers?.total ?? 0)
    : bindings.length;
  return (
    <Section label="Start bindings" data-testid="loop-start-bindings">
      <div className="rounded-lg border border-line bg-canvas-soft">
        <div className="flex flex-wrap items-center gap-1.5 px-3.5 py-3">
          {declaredKinds.map(kind => (
            <MonoTag
              key={kind}
              className="rounded-xs bg-badge-fill px-1.5 py-0.5"
              data-testid="loop-declared-kind"
            >
              {kind}
            </MonoTag>
          ))}
          <span className="ml-auto font-mono text-pill-group-badge text-faint">declared</span>
        </div>

        {hasPagination ? (
          <p
            className="border-t border-line-soft px-3.5 py-2 font-mono text-mono-id text-faint"
            data-testid="loop-bindings-progress"
          >
            {loadedBindings} of {totalBindings} attached loaded
          </p>
        ) : null}

        {isLoading ? (
          <div className="flex items-center gap-2 border-t border-line-soft px-3.5 py-3 text-form-hint text-subtle">
            <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
            Loading attached automations…
          </div>
        ) : bindings.length === 0 ? (
          <p
            className="border-t border-line-soft px-3.5 py-3 text-form-hint leading-relaxed text-subtle"
            data-testid="loop-bindings-empty"
          >
            Runs on demand. Attach an automation to one of the declared start kinds above to run
            this Loop hands-free.
          </p>
        ) : (
          bindings.map(binding => (
            <div
              key={binding.id}
              className="border-t border-line-soft px-3.5 py-2.5"
              data-testid="loop-binding-row"
              data-enabled={binding.enabled}
            >
              <div className="flex min-w-0 items-center gap-2">
                <span
                  aria-hidden="true"
                  className={`size-1.5 shrink-0 rounded-full ${
                    binding.enabled ? "bg-success" : "bg-neutral"
                  }`}
                  title={binding.enabled ? "Enabled" : "Disabled"}
                />
                <span
                  className={`min-w-0 truncate font-mono text-xs ${
                    binding.enabled ? "text-fg-strong" : "text-muted"
                  }`}
                >
                  {binding.name}
                </span>
                <MonoTag className="ml-auto shrink-0 rounded-xs bg-badge-fill px-1.5 py-0.5">
                  {bindingKindLabel(binding.kind)}
                </MonoTag>
              </div>
              <div className="mt-1.5 pl-3.5 font-mono text-mono-id text-subtle">{binding.meta}</div>
            </div>
          ))
        )}

        {jobs?.error ? (
          <p
            className="border-t border-line-soft px-3.5 py-2 text-caption text-danger"
            role="alert"
          >
            {jobs.error.message}
          </p>
        ) : null}
        {triggers?.error ? (
          <p
            className="border-t border-line-soft px-3.5 py-2 text-caption text-danger"
            role="alert"
          >
            {triggers.error.message}
          </p>
        ) : null}
        {jobs?.hasMore || triggers?.hasMore ? (
          <div className="flex flex-wrap gap-2 border-t border-line-soft px-3.5 py-2.5">
            {jobs?.hasMore ? (
              <Button
                aria-busy={jobs.isFetchingMore}
                data-testid="loop-bindings-load-more-jobs"
                disabled={jobs.isFetchingMore}
                onClick={jobs.loadMore}
                size="sm"
                type="button"
                variant="ghost"
              >
                {jobs.isFetchingMore ? "Loading schedules…" : "Load more schedules"}
              </Button>
            ) : null}
            {triggers?.hasMore ? (
              <Button
                aria-busy={triggers.isFetchingMore}
                data-testid="loop-bindings-load-more-triggers"
                disabled={triggers.isFetchingMore}
                onClick={triggers.loadMore}
                size="sm"
                type="button"
                variant="ghost"
              >
                {triggers.isFetchingMore ? "Loading triggers…" : "Load more triggers"}
              </Button>
            ) : null}
          </div>
        ) : null}

        {canAddTrigger || canAddSchedule ? (
          <div className="flex gap-2 border-t border-line-soft px-3.5 py-2.5">
            {canAddTrigger ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                data-testid="loop-add-trigger"
                onClick={onAddTrigger}
              >
                <Zap aria-hidden="true" className="size-3" />
                Add trigger
              </Button>
            ) : null}
            {canAddSchedule ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                data-testid="loop-add-schedule"
                onClick={onAddSchedule}
              >
                <CalendarClock aria-hidden="true" className="size-3" />
                Add schedule
              </Button>
            ) : null}
          </div>
        ) : (
          <div className="flex items-center gap-1.5 border-t border-line-soft px-3.5 py-2.5 text-badge text-faint">
            <Plus aria-hidden="true" className="size-3" />
            This Loop declares no automatable start kind.
          </div>
        )}
      </div>
    </Section>
  );
}
