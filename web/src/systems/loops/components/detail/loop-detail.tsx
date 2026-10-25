import { useState } from "react";
import {
  ArrowRight,
  ChevronDown,
  Clock3,
  Copy,
  History,
  PencilLine,
  Play,
  Radio,
  SlidersHorizontal,
  Trash2,
  Workflow,
  Zap,
} from "lucide-react";
import { Link, useNavigate } from "@tanstack/react-router";

import {
  Button,
  buttonVariants,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  HelpTip,
  SkeletonRegion,
  TopbarOverflowIcon,
  useTopbarSlot,
} from "@compozy/ui";

import { LoopPageLede } from "../loop-page-lede";
import { LoopSection } from "../loop-section";

import {
  loopAutomateStarts,
  type LoopAutomateStart,
  type LoopBindingRow,
} from "../../lib/loop-bindings";
import { loopMonthActivity, loopSourceLabel } from "../../lib/loop-catalog";
import type { LoopGraph } from "../../lib/loop-graph";
import type {
  LoopAggregate30d,
  LoopDetail as LoopDetailData,
  LoopEffectiveConfig,
  LoopRun,
} from "../../types";
import { LoopBodyDag } from "./loop-body-dag";
import { LoopContractPanel } from "./loop-contract-panel";
import { LoopDeclaredInputs } from "./loop-declared-inputs";
import { LoopDeleteAction } from "./loop-delete-action";
import { LoopDetailFrame } from "./loop-detail-frame";
import { loopDetailTopbarIdentity } from "./loop-detail-topbar";
import { LoopLimitsPanel } from "./loop-limits-panel";
import { LoopRecentRuns } from "./loop-recent-runs";
import { LoopRecentRunsSkeleton } from "./loop-recent-runs-skeleton";
import { LoopStartBindingsPanel, type LoopBindingPagination } from "./loop-start-bindings-panel";

interface LoopDetailProps {
  loop: LoopDetailData;
  effectiveConfig: LoopEffectiveConfig;
  graph: LoopGraph;
  recentRuns: readonly LoopRun[];
  /** The recent-runs read has not answered; the list shows its skeleton, not "has not run yet". */
  recentRunsLoading?: boolean;
  bindings: readonly LoopBindingRow[];
  bindingsLoading: boolean;
  bindingJobs?: LoopBindingPagination;
  bindingTriggers?: LoopBindingPagination;
  successRate: number | null;
  aggregate: LoopAggregate30d | null;
  onRun: () => void;
  onConfigure: () => void;
  onOpenEditor: () => void;
  onDelete: () => Promise<void>;
  onDeleteReset: () => void;
  deletePending: boolean;
  deleteError: string | null;
  /** Opens the Automations editor locked to this Loop with the chosen start. */
  onAutomate: (start: LoopAutomateStart) => void;
}

export function LoopDetailView({
  loop,
  effectiveConfig,
  graph,
  recentRuns,
  recentRunsLoading = false,
  bindings,
  bindingsLoading,
  bindingJobs,
  bindingTriggers,
  successRate,
  aggregate,
  onRun,
  onConfigure,
  onOpenEditor,
  onDelete,
  onDeleteReset,
  deletePending,
  deleteError,
  onAutomate,
}: LoopDetailProps) {
  const definition = loop.definition;
  const category = loop.catalog?.category;
  const sourceLabel = loopSourceLabel(loop);
  const writable = loop.source === "workspace";
  const declaredKinds = (definition.start ?? []).map(binding => binding.kind);
  const navigate = useNavigate();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const backToLoops = () => {
    void navigate({ to: "/loops" });
  };
  useTopbarSlot({
    ...loopDetailTopbarIdentity(loop.name, backToLoops),
    actions: (
      <div className="flex items-center gap-1.5" data-testid="loop-detail-actions">
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={onRun}
          data-testid="loop-run-action"
        >
          <Play aria-hidden="true" className="size-3.5" />
          Run loop
        </Button>
        <LoopAutomateMenu starts={loopAutomateStarts(declaredKinds)} onAutomate={onAutomate} />
      </div>
    ),
    overflow: (
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label="More actions"
          data-testid="loop-detail-overflow"
          render={<Button type="button" variant="ghost" size="icon-sm" />}
        >
          <TopbarOverflowIcon aria-hidden="true" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" data-testid="loop-detail-overflow-menu">
          <DropdownMenuItem data-testid="loop-edit-action" onClick={onOpenEditor}>
            {writable ? (
              <PencilLine aria-hidden="true" className="size-3.5" />
            ) : (
              <Copy aria-hidden="true" className="size-3.5" />
            )}
            {writable ? "Edit" : "Copy and edit"}
          </DropdownMenuItem>
          <DropdownMenuItem data-testid="loop-configure-action" onClick={onConfigure}>
            <SlidersHorizontal aria-hidden="true" className="size-3.5" />
            Configure
          </DropdownMenuItem>
          {writable ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuItem
                data-testid="loop-delete-action"
                onClick={() => setDeleteOpen(true)}
                variant="destructive"
              >
                <Trash2 aria-hidden="true" className="size-3.5" />
                Delete loop
              </DropdownMenuItem>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    ),
  });
  return (
    <LoopDetailFrame
      data-testid="loop-detail"
      lede={
        <LoopPageLede
          lede={loop.description}
          meta={[
            ...(category ? [category] : []),
            `${graph.nodes.length} ${graph.nodes.length === 1 ? "step" : "steps"}`,
            ...(aggregate && successRate !== null
              ? [detailActivityLine(aggregate.runs, successRate)]
              : []),
          ]}
          name={loop.name}
          tags={[sourceLabel, `v${loop.version}`]}
          testId="loop-detail-header"
        />
      }
      main={
        <>
          <LoopContractPanel contract={definition.contract} concurrency={definition.concurrency} />
          <LoopSection
            icon={<Workflow aria-hidden="true" />}
            right={
              <span className="flex items-center gap-1">
                {writable ? null : (
                  <HelpTip label="About copying built-in Loops">
                    Built-in Loops are read-only. Editing makes your own copy in this project.
                  </HelpTip>
                )}
                <Button
                  data-testid="loop-open-builder"
                  onClick={onOpenEditor}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  {writable ? <PencilLine aria-hidden="true" /> : <Copy aria-hidden="true" />}
                  {writable ? "Edit steps" : "Copy and edit"}
                </Button>
              </span>
            }
            title="Steps"
          >
            <LoopBodyDag graph={graph} />
          </LoopSection>
          <LoopSection
            icon={<History aria-hidden="true" />}
            right={
              <Link
                to="/loop-runs"
                className={buttonVariants({ size: "sm", variant: "ghost" })}
                data-testid="loop-all-runs"
              >
                All Loop runs
                <ArrowRight aria-hidden="true" className="size-3" />
              </Link>
            }
            title="Recent runs"
          >
            {recentRunsLoading ? (
              <SkeletonRegion label="Loading recent runs">
                <LoopRecentRunsSkeleton />
              </SkeletonRegion>
            ) : (
              <LoopRecentRuns runs={recentRuns} />
            )}
          </LoopSection>
        </>
      }
      rail={
        <>
          <LoopDeclaredInputs inputs={definition.inputs} />
          <LoopStartBindingsPanel
            loopName={loop.name}
            declaredKinds={declaredKinds}
            bindings={bindings}
            jobs={bindingJobs}
            isLoading={bindingsLoading}
            triggers={bindingTriggers}
          />
          <LoopLimitsPanel effectiveConfig={effectiveConfig} />
        </>
      }
    >
      {writable ? (
        <LoopDeleteAction
          error={deleteError}
          hideTrigger
          isPending={deletePending}
          loopName={loop.name}
          onConfirm={onDelete}
          onOpenChange={setDeleteOpen}
          onReset={onDeleteReset}
          open={deleteOpen}
        />
      ) : null}
    </LoopDetailFrame>
  );
}

const AUTOMATE_ITEMS = {
  schedule: { icon: Clock3, label: "On a schedule" },
  event: { icon: Radio, label: "When something happens" },
} as const;

/** One "Automate ▾" menu with the starts the Loop's allowlist permits; absent when none. */
function LoopAutomateMenu({
  starts,
  onAutomate,
}: {
  starts: readonly LoopAutomateStart[];
  onAutomate: (start: LoopAutomateStart) => void;
}) {
  if (starts.length === 0) return null;
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        data-testid="loop-automate-action"
        render={<Button type="button" variant="ghost" size="sm" />}
      >
        <Zap aria-hidden="true" className="size-3.5" />
        Automate
        <ChevronDown aria-hidden="true" className="size-3" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" data-testid="loop-automate-menu">
        {starts.map(start => {
          const { icon: Icon, label } = AUTOMATE_ITEMS[start];
          return (
            <DropdownMenuItem
              data-testid={`loop-automate-${start}`}
              key={start}
              onClick={() => onAutomate(start)}
            >
              <Icon aria-hidden="true" className="size-3.5" />
              {label}
            </DropdownMenuItem>
          );
        })}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/** The lede's 30-day line; a Loop with no runs states that instead of "0% success". */
function detailActivityLine(runs: number, successRate: number): string {
  const activity = loopMonthActivity(runs, successRate);
  return activity.hasRuns ? `${activity.rate} success · ${activity.runs}` : activity.runs;
}
