import { useState, type ComponentProps } from "react";
import { Workflow } from "@compozy/ui/icons";

import { PAGE_CONTENT_GUTTER, cn, useTopbarSlot } from "@compozy/ui";

import { automationDeleteConsequence, describeScheduleStarts } from "../../lib/automation-detail";
import { isAutomationTrigger, type AutomationEntity } from "../../lib/automation-entity";
import { triggerEventLabel } from "../../lib/automation-rule";
import { projectAutomationTarget } from "../../lib/automation-target";
import type { SentenceContext } from "../../lib/automation-sentence";
import type { AutomationView } from "../../lib/automation-view";
import type { AutomationRun } from "../../types";
import { AutomationDeleteAction } from "../automation-delete-action";
import { AutomationDetailActions, AutomationDetailOverflow } from "./automation-detail-actions";
import { AutomationDetailHead } from "./automation-detail-head";
import { AutomationDetailSection } from "./automation-detail-section";
import { AutomationDetailSkeleton } from "./automation-detail-skeleton";
import {
  AutomationDetailUnavailable,
  type AutomationDetailUnavailableReason,
} from "./automation-detail-states";
import { AutomationInspectSheet } from "./automation-inspect-sheet";
import { AutomationLockbar } from "./automation-lockbar";
import { AutomationRail } from "./automation-rail";
import { AutomationRuleCard } from "./automation-rule-card";
import { AutomationRunList } from "./automation-run-list";
import type { WorkspaceBadgeIdentity } from "@/systems/workspace";

export type AutomationDetailStatus = "loading" | "ready" | AutomationDetailUnavailableReason;

export interface AutomationDetailState {
  isDeleting: boolean;
  isTogglePending: boolean;
  isRunNowPending: boolean;
  /** Automations are unavailable, so Run now would be refused. */
  isRunNowDisabled: boolean;
}

export interface AutomationDetailPanelProps {
  status: AutomationDetailStatus;
  /** The entity the route names; the loading skeleton holds the bands that kind has. */
  kind?: "job" | "trigger";
  /** The routed id: the topbar names it until the automation's own name arrives. */
  entityId?: string;
  /** Shown by the missing / other-project / error states. */
  statusMessage: string;
  entity: AutomationEntity | undefined;
  view: AutomationView | undefined;
  sentenceContext: SentenceContext;
  /** Display name of a Loop target's project. */
  loopWorkspaceName: string | null;
  /** The Loop target no longer exists in its project. */
  loopMissing: boolean;
  lastRanAt: string | null;
  runs: AutomationRun[];
  runsError: Error | null;
  runsLoading: boolean;
  /** Under All projects: the project each run is attributed to. */
  runProject?: (run: AutomationRun) => WorkspaceBadgeIdentity;
  state: AutomationDetailState;
  onBack: () => void;
  onDelete: () => void | Promise<void>;
  onEdit: () => void;
  onRetryRuns: () => void;
  onRunNow: () => void;
  onSetUpRetries: () => void;
  onToggleEnabled: (enabled: boolean) => void;
}

/**
 * One detail page for every automation: the sentence with its switch, "How
 * it works", the runs, and one rail card — the same grammar whether it
 * starts on a schedule, on an event or from a link.
 */
export function AutomationDetailPanel(props: AutomationDetailPanelProps) {
  // `status`/`statusMessage` belong to the unavailable states; the loaded page spreads the rest.
  const { status, statusMessage, kind, entityId, ...loaded } = props;
  const { entity, view, onBack } = loaded;
  // Transient overlays live above the branch that unmounts: a refetch that
  // briefly empties the record must not slam a sheet shut mid-read.
  const [deleteOverlay, setDeleteOverlay] = useState({ open: false, id: "" });
  const [inspectOverlay, setInspectOverlay] = useState({ open: false, id: "" });
  const id = entity?.id ?? "";

  if (status === "loading") {
    return (
      <>
        <AutomationDetailLoadingChrome entityId={entityId} onBack={onBack} />
        <AutomationDetailSkeleton kind={kind} />
      </>
    );
  }
  if (status !== "ready" || !entity || !view) {
    return (
      <AutomationDetailUnavailable
        description={statusMessage}
        onBack={onBack}
        reason={status === "ready" ? "missing" : status}
      />
    );
  }

  return (
    <AutomationDetailLoaded
      {...loaded}
      deleteOpen={deleteOverlay.open && deleteOverlay.id === id}
      entity={entity}
      inspectOpen={inspectOverlay.open && inspectOverlay.id === id}
      onDeleteOpenChange={open => setDeleteOverlay({ open, id })}
      onInspectOpenChange={open => setInspectOverlay({ open, id })}
      view={view}
    />
  );
}

/** The topbar while the automation loads: the way back and the routed id, as the router names it. */
function AutomationDetailLoadingChrome({
  entityId,
  onBack,
}: {
  entityId?: string;
  onBack: () => void;
}) {
  useTopbarSlot({
    onBack,
    crumbs: [{ id: "catalog", label: "Automations", onSelect: onBack }],
    crumb: entityId,
  });
  return null;
}

type LoadedProps = Omit<
  AutomationDetailPanelProps,
  "entity" | "view" | "status" | "statusMessage" | "kind" | "entityId"
> &
  Omit<ComponentProps<"section">, "children"> & {
    entity: AutomationEntity;
    view: AutomationView;
    deleteOpen: boolean;
    inspectOpen: boolean;
    onDeleteOpenChange: (open: boolean) => void;
    onInspectOpenChange: (open: boolean) => void;
  };

/** `Weekdays 09:00 → summarizer` / `Session stopped → software-delivery`. */
function ruleGist(entity: AutomationEntity, ctx: SentenceContext): string {
  const starts = isAutomationTrigger(entity)
    ? triggerEventLabel(entity)
    : describeScheduleStarts(entity, ctx).headline;
  const target = projectAutomationTarget(entity);
  const does =
    !isAutomationTrigger(entity) && entity.task
      ? "a task"
      : target.kind === "loop"
        ? target.loopName
        : target.agentName;
  return `${starts} → ${does}`;
}

function AutomationDetailLoaded({
  entity,
  view,
  sentenceContext,
  loopWorkspaceName,
  loopMissing,
  lastRanAt,
  runs,
  runsError,
  runsLoading,
  runProject,
  state,
  deleteOpen,
  inspectOpen,
  onBack,
  onDelete,
  onDeleteOpenChange,
  onEdit,
  onInspectOpenChange,
  onRetryRuns,
  onRunNow,
  onSetUpRetries,
  onToggleEnabled,
  className,
  ...props
}: LoadedProps) {
  useTopbarSlot({
    onBack,
    crumbs: [{ id: "catalog", label: "Automations", onSelect: onBack }],
    crumb: view.name,
    actions: (
      <AutomationDetailActions
        onEdit={onEdit}
        onRunNow={onRunNow}
        runNow={{ disabled: state.isRunNowDisabled, pending: state.isRunNowPending }}
        view={view}
      />
    ),
    overflow: (
      <AutomationDetailOverflow
        onDelete={() => onDeleteOpenChange(true)}
        onInspect={() => onInspectOpenChange(true)}
        view={view}
      />
    ),
  });

  return (
    <section
      className={cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col overflow-hidden", className)}
      data-testid="automation-detail-panel"
      {...props}
    >
      {view.canEdit ? (
        <AutomationDeleteAction
          consequence={automationDeleteConsequence(entity)}
          hideTrigger
          isPending={state.isDeleting}
          key={`delete:${entity.id}`}
          name={view.name}
          onConfirm={onDelete}
          onOpenChange={onDeleteOpenChange}
          open={deleteOpen}
        />
      ) : null}

      <div className="@container min-h-0 flex-1 overflow-y-auto pt-5 pb-16">
        <AutomationLockbar view={view} />
        <AutomationDetailHead
          isTogglePending={state.isTogglePending}
          lastRanAt={lastRanAt}
          onToggleEnabled={onToggleEnabled}
          updatedAt={entity.updated_at}
          view={view}
        />
        <div className="grid items-start gap-8 @3xl:grid-cols-[minmax(0,1fr)_var(--width-detail-inspector-inline)]">
          <main className="flex min-w-0 flex-col gap-6">
            <AutomationDetailSection
              data-testid="automation-rule-section"
              gist={ruleGist(entity, sentenceContext)}
              icon={Workflow}
              label="How it works"
            >
              <AutomationRuleCard
                entity={entity}
                loopMissing={loopMissing}
                loopWorkspaceName={loopWorkspaceName}
                sentenceContext={sentenceContext}
                view={view}
              />
            </AutomationDetailSection>
            <AutomationRunList
              entity={entity}
              error={runsError}
              isLoading={runsLoading}
              nextRunAt={view.enabled ? view.nextRunAt : undefined}
              onRetry={onRetryRuns}
              onSetUpRetries={view.canEdit ? onSetUpRetries : undefined}
              runProject={runProject}
              runs={runs}
            />
          </main>
          <AutomationRail
            entity={entity}
            lastRanAt={lastRanAt}
            loopWorkspaceName={loopWorkspaceName}
            onInspect={() => onInspectOpenChange(true)}
            sentenceContext={sentenceContext}
            view={view}
          />
        </div>
      </div>

      <AutomationInspectSheet
        entity={entity}
        key={`inspect:${entity.id}`}
        onOpenChange={onInspectOpenChange}
        open={inspectOpen}
      />
    </section>
  );
}
