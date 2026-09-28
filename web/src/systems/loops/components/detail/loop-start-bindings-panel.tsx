import { CalendarClock, Plus, Zap } from "lucide-react";

import { Button, Pill, Spinner, StatusDot } from "@compozy/ui";

import { bindingKindLabel, type LoopBindingRow } from "../../lib/loop-bindings";
import { LoopRailSection } from "../loop-rail-section";

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
  const hasMore = Boolean(jobs?.hasMore || triggers?.hasMore);
  const loadedBindings = hasPagination
    ? (jobs?.loaded ?? 0) + (triggers?.loaded ?? 0)
    : bindings.length;
  const totalBindings = hasPagination
    ? (jobs?.total ?? 0) + (triggers?.total ?? 0)
    : bindings.length;
  return (
    <LoopRailSection
      data-testid="loop-start-bindings"
      gist={
        totalBindings === 0
          ? "Manual only"
          : `${totalBindings} ${totalBindings === 1 ? "automation" : "automations"}`
      }
      icon={<Zap aria-hidden="true" className="size-3.5" />}
      title="Automations"
    >
      <div className="[&>*:first-child]:border-t-0">
        {hasPagination && hasMore ? (
          <p
            className="border-t border-line-soft px-4 py-2 text-form-hint text-faint"
            data-testid="loop-bindings-progress"
          >
            Showing {loadedBindings} of {totalBindings}
          </p>
        ) : null}

        {isLoading ? (
          <div className="flex items-center gap-2 border-t border-line-soft px-4 py-3 text-form-hint text-subtle">
            <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
            Loading attached automations…
          </div>
        ) : bindings.length === 0 ? (
          <p
            className="border-t border-line-soft px-4 py-3 text-form-hint leading-relaxed text-subtle"
            data-testid="loop-bindings-empty"
          >
            Runs only when you start it.
          </p>
        ) : (
          bindings.map(binding => (
            <div
              key={binding.id}
              className="border-t border-line-soft px-4 py-2.5"
              data-testid="loop-binding-row"
              data-enabled={binding.enabled}
            >
              <div className="flex min-w-0 items-center gap-2">
                <StatusDot
                  label={binding.enabled ? "On" : "Off"}
                  size="sm"
                  tone={binding.enabled ? "success" : "faint"}
                />
                <span
                  className={`min-w-0 truncate text-form-label ${
                    binding.enabled ? "text-fg-strong" : "text-muted"
                  }`}
                >
                  {binding.name}
                </span>
                <Pill className="ml-auto shrink-0" size="xs" tone="neutral">
                  {bindingKindLabel(binding.kind)}
                </Pill>
              </div>
              <div className="mt-1 pl-3.5 text-form-hint text-subtle">{binding.meta}</div>
            </div>
          ))
        )}

        {jobs?.error ? (
          <p className="border-t border-line-soft px-4 py-2 text-caption text-danger" role="alert">
            {jobs.error.message}
          </p>
        ) : null}
        {triggers?.error ? (
          <p className="border-t border-line-soft px-4 py-2 text-caption text-danger" role="alert">
            {triggers.error.message}
          </p>
        ) : null}
        {jobs?.hasMore || triggers?.hasMore ? (
          <div className="flex flex-wrap gap-2 border-t border-line-soft px-4 py-2.5">
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
          <div className="flex gap-2 border-t border-line-soft px-4 py-2.5">
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
          <div className="flex items-center gap-1.5 border-t border-line-soft px-4 py-2.5 text-badge text-faint">
            <Plus aria-hidden="true" className="size-3" />
            This Loop can only be started by hand.
          </div>
        )}
      </div>
    </LoopRailSection>
  );
}
