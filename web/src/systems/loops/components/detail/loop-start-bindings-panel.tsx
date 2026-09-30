import { CalendarClock, Plus, Zap } from "lucide-react";

import { Button, Pill, Spinner, StatusDot } from "@compozy/ui";

import {
  bindingKindLabel,
  bindingsGist,
  countLoopBindings,
  type LoopBindingRow,
} from "../../lib/loop-bindings";
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
  const counts = countLoopBindings(bindings.length, jobs, triggers);
  return (
    <LoopRailSection
      data-testid="loop-start-bindings"
      gist={bindingsGist(counts.total)}
      icon={<Zap aria-hidden="true" className="size-3.5" />}
      title="Automations"
    >
      <div className="[&>*:first-child]:border-t-0">
        {counts.paginated && counts.hasMore ? (
          <p
            className="border-t border-line-soft px-4 py-2 text-form-hint text-faint"
            data-testid="loop-bindings-progress"
          >
            Showing {counts.loaded} of {counts.total}
          </p>
        ) : null}
        <LoopBindingsList bindings={bindings} isLoading={isLoading} />
        <LoopBindingsPageError pagination={jobs} />
        <LoopBindingsPageError pagination={triggers} />
        <LoopBindingsLoadMore jobs={jobs} triggers={triggers} />
        <LoopBindingsAddActions
          declaredKinds={declaredKinds}
          onAddSchedule={onAddSchedule}
          onAddTrigger={onAddTrigger}
        />
      </div>
    </LoopRailSection>
  );
}

function LoopBindingsList({
  bindings,
  isLoading,
}: {
  bindings: readonly LoopBindingRow[];
  isLoading: boolean;
}) {
  if (isLoading) {
    return (
      <div className="flex items-center gap-2 border-t border-line-soft px-4 py-3 text-form-hint text-subtle">
        <Spinner aria-hidden="true" className="size-3.5 text-subtle" />
        Loading attached automations…
      </div>
    );
  }
  if (bindings.length === 0) {
    return (
      <p
        className="border-t border-line-soft px-4 py-3 text-form-hint leading-relaxed text-subtle"
        data-testid="loop-bindings-empty"
      >
        Runs only when you start it.
      </p>
    );
  }
  return bindings.map(binding => <LoopBindingItem key={binding.id} binding={binding} />);
}

function LoopBindingItem({ binding }: { binding: LoopBindingRow }) {
  return (
    <div
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
  );
}

function LoopBindingsPageError({ pagination }: { pagination?: LoopBindingPagination }) {
  if (!pagination?.error) return null;
  return (
    <p className="border-t border-line-soft px-4 py-2 text-caption text-danger" role="alert">
      {pagination.error.message}
    </p>
  );
}

function LoopBindingsLoadMore({
  jobs,
  triggers,
}: {
  jobs?: LoopBindingPagination;
  triggers?: LoopBindingPagination;
}) {
  if (!jobs?.hasMore && !triggers?.hasMore) return null;
  return (
    <div className="flex flex-wrap gap-2 border-t border-line-soft px-4 py-2.5">
      {jobs?.hasMore ? (
        <LoadMoreButton
          idleLabel="Load more schedules"
          loadingLabel="Loading schedules…"
          pagination={jobs}
          testId="loop-bindings-load-more-jobs"
        />
      ) : null}
      {triggers?.hasMore ? (
        <LoadMoreButton
          idleLabel="Load more triggers"
          loadingLabel="Loading triggers…"
          pagination={triggers}
          testId="loop-bindings-load-more-triggers"
        />
      ) : null}
    </div>
  );
}

function LoadMoreButton({
  pagination,
  idleLabel,
  loadingLabel,
  testId,
}: {
  pagination: LoopBindingPagination;
  idleLabel: string;
  loadingLabel: string;
  testId: string;
}) {
  return (
    <Button
      aria-busy={pagination.isFetchingMore}
      data-testid={testId}
      disabled={pagination.isFetchingMore}
      onClick={pagination.loadMore}
      size="sm"
      type="button"
      variant="ghost"
    >
      {pagination.isFetchingMore ? loadingLabel : idleLabel}
    </Button>
  );
}

function LoopBindingsAddActions({
  declaredKinds,
  onAddTrigger,
  onAddSchedule,
}: Pick<LoopStartBindingsPanelProps, "declaredKinds" | "onAddTrigger" | "onAddSchedule">) {
  const canAddSchedule = declaredKinds.includes(SCHEDULE_KIND);
  const canAddTrigger = declaredKinds.some(kind => TRIGGER_KINDS.has(kind));
  if (!canAddTrigger && !canAddSchedule) {
    return (
      <div className="flex items-center gap-1.5 border-t border-line-soft px-4 py-2.5 text-badge text-faint">
        <Plus aria-hidden="true" className="size-3" />
        This Loop can only be started by hand.
      </div>
    );
  }
  return (
    <div className="flex gap-2 border-t border-line-soft px-4 py-2.5">
      {canAddTrigger ? (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          data-testid="loop-add-trigger"
          onClick={onAddTrigger}
        >
          <Zap aria-hidden="true" />
          Add trigger
        </Button>
      ) : null}
      {canAddSchedule ? (
        <Button
          type="button"
          variant="secondary"
          size="sm"
          data-testid="loop-add-schedule"
          onClick={onAddSchedule}
        >
          <CalendarClock aria-hidden="true" />
          Add schedule
        </Button>
      ) : null}
    </div>
  );
}
