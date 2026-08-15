import { useState, type ComponentProps } from "react";
import { Zap } from "lucide-react";

import { Button, PAGE_CONTENT_GUTTER, cn, useTopbarSlot } from "@compozy/ui";

import { triggerTargetName } from "../../lib/trigger-sentence";
import type { AutomationRun, AutomationTrigger } from "../../types";
import { AutomationDeleteAction } from "../automation-delete-action";
import { TriggerDetailActions, TriggerDetailOverflow } from "./trigger-detail-actions";
import { TriggerDetailHead } from "./trigger-detail-head";
import { TriggerDetailLock } from "./trigger-detail-lock";
import { TriggerDetailSection } from "./trigger-detail-section";
import { TriggerDetailRail } from "./trigger-detail-rail";
import { TriggerDetailSkeleton, TriggerDetailUnavailable } from "./trigger-detail-states";
import { TriggerInspectSheet } from "./trigger-inspect-sheet";
import { TriggerRuleCard } from "./trigger-rule-card";
import { TriggerRunList } from "./trigger-run-list";

interface TriggerDetailState {
  isDeleting: boolean;
  isLoading: boolean;
  isTogglePending: boolean;
}

export interface TriggerDetailPanelProps {
  error: Error | null;
  trigger: AutomationTrigger | undefined;
  /** Resolved display name for the owning workspace; falls back to its id. */
  workspaceName: string | null;
  /** Resolved display name for a loop target's workspace. */
  loopWorkspaceName: string | null;
  state: TriggerDetailState;
  onBack: () => void;
  onDelete: () => void | Promise<void>;
  onEdit: () => void;
  onRetryRuns: () => void;
  onToggleEnabled: (enabled: boolean) => void;
  runs: AutomationRun[];
  runsError: Error | null;
  runsLoading: boolean;
}

/**
 * The trigger detail surface.
 *
 * A trigger is a When → If → Then rule, so this page answers three questions in
 * that order: what it watches, whether it is on, and whether the last fire
 * worked. It is deliberately not the shared automation inspector with the
 * schedule removed — there is no next run, no fire button, and no metric the
 * daemon cannot back.
 */
export function TriggerDetailPanel({
  error,
  trigger,
  workspaceName,
  loopWorkspaceName,
  state,
  onBack,
  onDelete,
  onEdit,
  onRetryRuns,
  onToggleEnabled,
  runs,
  runsError,
  runsLoading,
}: TriggerDetailPanelProps) {
  // Transient panel state lives here, above the branch that unmounts: a refetch
  // that briefly empties the record must not slam a sheet shut mid-read.
  const [deleteOverlay, setDeleteOverlay] = useState({ open: false, triggerId: "" });
  const [inspectOverlay, setInspectOverlay] = useState({ open: false, triggerId: "" });
  const triggerId = trigger?.id ?? "";
  const deleteOpen = deleteOverlay.open && deleteOverlay.triggerId === triggerId;
  const inspectOpen = inspectOverlay.open && inspectOverlay.triggerId === triggerId;

  if (state.isLoading) {
    return <TriggerDetailSkeleton />;
  }

  if (error) {
    return (
      <TriggerDetailUnavailable
        description={error.message || "Failed to load this trigger."}
        reason="error"
      />
    );
  }

  if (!trigger) {
    return (
      <TriggerDetailUnavailable
        action={
          <Button onClick={onBack} size="sm" type="button" variant="neutral">
            Back to Triggers
          </Button>
        }
        description="This trigger is no longer available."
        reason="gone"
      />
    );
  }

  return (
    <TriggerDetailLoadedPanel
      deleteOpen={deleteOpen}
      inspectOpen={inspectOpen}
      loopWorkspaceName={loopWorkspaceName}
      onBack={onBack}
      onDelete={onDelete}
      onDeleteOpenChange={open => setDeleteOverlay({ open, triggerId })}
      onEdit={onEdit}
      onInspectOpenChange={open => setInspectOverlay({ open, triggerId })}
      onRetryRuns={onRetryRuns}
      onToggleEnabled={onToggleEnabled}
      runs={runs}
      runsError={runsError}
      runsLoading={runsLoading}
      state={{
        isDeleting: state.isDeleting,
        isTogglePending: state.isTogglePending,
      }}
      trigger={trigger}
      workspaceName={workspaceName}
    />
  );
}

type LoadedPanelProps = Omit<TriggerDetailPanelProps, "error" | "state" | "trigger"> &
  Omit<ComponentProps<"section">, "children"> & {
    trigger: AutomationTrigger;
    state: Omit<TriggerDetailState, "isLoading">;
    deleteOpen: boolean;
    inspectOpen: boolean;
    onDeleteOpenChange: (open: boolean) => void;
    onInspectOpenChange: (open: boolean) => void;
  };

/** Newest actual run start, excluding scheduled reservations that have not begun. */
function lastRanAt(runs: AutomationRun[]): string | null {
  let newest: string | null = null;
  for (const run of runs) {
    if (run.status === "scheduled") continue;
    const startedAt = run.started_at;
    if (!startedAt) continue;
    if (newest === null || startedAt > newest) newest = startedAt;
  }
  return newest;
}

function ruleGist(trigger: AutomationTrigger): string {
  const target = triggerTargetName(trigger);
  const slug = trigger.endpoint_slug?.trim();
  const source = trigger.event === "webhook" && slug ? `webhook · ${slug}` : trigger.event;
  return target ? `${source} → ${target}` : source;
}

function TriggerDetailLoadedPanel({
  trigger,
  workspaceName,
  loopWorkspaceName,
  state,
  deleteOpen,
  inspectOpen,
  onBack,
  onDelete,
  onDeleteOpenChange,
  onEdit,
  onInspectOpenChange,
  onRetryRuns,
  onToggleEnabled,
  runs,
  runsError,
  runsLoading,
  className,
  ...props
}: LoadedPanelProps) {
  useTopbarSlot({
    onBack,
    crumbs: [{ id: "catalog", label: "Triggers", onSelect: onBack }],
    crumb: trigger.name,
    actions:
      trigger.source === "dynamic" ? (
        <TriggerDetailActions onEdit={onEdit} trigger={trigger} />
      ) : undefined,
    overflow: (
      <TriggerDetailOverflow
        isTogglePending={state.isTogglePending}
        onDelete={() => onDeleteOpenChange(true)}
        onEdit={onEdit}
        onToggleEnabled={onToggleEnabled}
        trigger={trigger}
      />
    ),
  });

  return (
    <section
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden", className)}
      data-testid="automation-detail-panel"
      {...props}
    >
      {trigger.source === "dynamic" ? (
        <AutomationDeleteAction
          key={`delete:${trigger.id}`}
          hideTrigger
          isPending={state.isDeleting}
          kind="triggers"
          name={trigger.name}
          onConfirm={onDelete}
          onOpenChange={onDeleteOpenChange}
          open={deleteOpen}
        />
      ) : null}

      <div className="min-h-0 flex-1 overflow-y-auto pt-5 pb-10">
        <TriggerDetailLock trigger={trigger} />
        <TriggerDetailHead
          isTogglePending={state.isTogglePending}
          lastRanAt={lastRanAt(runs)}
          onToggleEnabled={onToggleEnabled}
          trigger={trigger}
          workspaceName={workspaceName}
        />
        <div className="grid items-start gap-8 lg:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
          <main className="flex min-w-0 flex-col gap-6">
            <TriggerDetailSection
              data-testid="trigger-rule-section"
              gist={ruleGist(trigger)}
              icon={Zap}
              label="Rule"
            >
              <TriggerRuleCard
                loopWorkspaceName={loopWorkspaceName}
                trigger={trigger}
                workspaceName={workspaceName}
              />
            </TriggerDetailSection>
            <TriggerRunList
              error={runsError}
              isLoading={runsLoading}
              onRetry={onRetryRuns}
              runs={runs}
              trigger={trigger}
            />
          </main>
          <TriggerDetailRail
            loopWorkspaceName={loopWorkspaceName}
            onInspect={() => onInspectOpenChange(true)}
            trigger={trigger}
          />
        </div>
      </div>

      <TriggerInspectSheet
        key={`inspect:${trigger.id}`}
        onOpenChange={onInspectOpenChange}
        open={inspectOpen}
        trigger={trigger}
      />
    </section>
  );
}
