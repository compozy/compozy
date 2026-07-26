import { useState } from "react";
import { ArrowRight, GitFork, PencilLine, Play, SlidersHorizontal } from "lucide-react";
import { Link, useNavigate } from "@tanstack/react-router";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  PAGE_CONTENT_GUTTER,
  Pill,
  Section,
  TopbarOverflowIcon,
  useTopbarSlot,
} from "@agh/ui";

import type { LoopBindingRow } from "../../lib/loop-bindings";
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
import { LoopLimitsPanel } from "./loop-limits-panel";
import { LoopRecentRuns } from "./loop-recent-runs";
import { LoopStartBindingsPanel, type LoopBindingPagination } from "./loop-start-bindings-panel";
import { LoopStatsPanel } from "./loop-stats-panel";
import { LoopVersionsPanel } from "./loop-versions-panel";

interface LoopDetailProps {
  loop: LoopDetailData;
  effectiveConfig: LoopEffectiveConfig;
  graph: LoopGraph;
  recentRuns: readonly LoopRun[];
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
  onAddTrigger: () => void;
  onAddSchedule: () => void;
}

/**
 * The Loop definition page (design §4.2): detail header + Run/Configure/Fork
 * actions over a two-column layout — contract, read-only body graph, and recent
 * runs on the left; declared inputs, start bindings, limits, versions, and 30d
 * stats on the right rail.
 */
export function LoopDetailView({
  loop,
  effectiveConfig,
  graph,
  recentRuns,
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
  onAddTrigger,
  onAddSchedule,
}: LoopDetailProps) {
  const definition = loop.definition;
  const category = loop.catalog?.category;
  const sourceLabel = loop.source === "workspace" ? "Workspace" : "Read-only";
  const writable = loop.source === "workspace";
  const declaredKinds = (definition.start ?? []).map(binding => binding.kind);
  const navigate = useNavigate();
  const [deleteOpen, setDeleteOpen] = useState(false);
  const backToLoops = () => {
    void navigate({ to: "/loops" });
  };
  useTopbarSlot({
    onBack: backToLoops,
    crumbs: [{ id: "loops", label: "Loops", onSelect: backToLoops }],
    crumb: loop.name,
    actions: (
      <div className="flex items-center" data-testid="loop-detail-actions">
        <Button type="button" size="sm" onClick={onRun} data-testid="loop-run-action">
          <Play aria-hidden="true" className="size-3.5" />
          Run loop
        </Button>
      </div>
    ),
    overflow: (
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label="More actions"
          data-testid="loop-detail-overflow"
          render={<Button type="button" variant="ghost" size="icon-sm" />}
        >
          <TopbarOverflowIcon aria-hidden="true" className="size-3" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" data-testid="loop-detail-overflow-menu">
          <DropdownMenuItem data-testid="loop-edit-action" onClick={onOpenEditor}>
            {writable ? (
              <PencilLine aria-hidden="true" className="size-3.5" />
            ) : (
              <GitFork aria-hidden="true" className="size-3.5" />
            )}
            {writable ? "Edit" : "Fork & edit"}
          </DropdownMenuItem>
          <DropdownMenuItem data-testid="loop-configure-action" onClick={onConfigure}>
            <SlidersHorizontal aria-hidden="true" className="size-3.5" />
            Configure
          </DropdownMenuItem>
          {writable ? (
            <DropdownMenuItem
              data-testid="loop-delete-action"
              onClick={() => setDeleteOpen(true)}
              variant="destructive"
            >
              Delete loop
            </DropdownMenuItem>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
    ),
  });
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto" data-testid="loop-detail">
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
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col")}>
        <div className="flex flex-col gap-2 pt-4" data-testid="loop-detail-header">
          <div className="flex flex-wrap items-center gap-1.5">
            <Pill size="xs" tone="neutral">
              {sourceLabel}
            </Pill>
            <Pill size="xs" tone="neutral">
              v{loop.version}
            </Pill>
          </div>
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-small-body text-subtle">
            <span className="font-mono">{definition.apiVersion}</span>
            {category ? <span>{category}</span> : null}
            <span>{graph.nodes.length} nodes</span>
            {loop.description ? <span className="truncate">{loop.description}</span> : null}
          </div>
        </div>
        <div className="py-6">
          <div className="grid grid-cols-1 gap-8 lg:grid-cols-[minmax(0,1fr)_320px]">
            <div className="flex flex-col gap-7">
              <LoopContractPanel
                contract={definition.contract}
                concurrency={definition.concurrency}
              />
              <Section
                label="Body · DAG"
                right={
                  <button
                    type="button"
                    onClick={onOpenEditor}
                    className="inline-flex items-center gap-1.5 text-form-hint font-medium text-muted transition-colors hover:text-fg-strong"
                    data-testid="loop-open-builder"
                  >
                    Open in builder
                    <ArrowRight aria-hidden="true" className="size-3" />
                  </button>
                }
              >
                <LoopBodyDag graph={graph} />
              </Section>
              <Section
                label="Recent runs"
                right={
                  <Link
                    to="/loop-runs"
                    className="inline-flex items-center gap-1.5 text-form-hint font-medium text-muted transition-colors hover:text-fg-strong"
                    data-testid="loop-all-runs"
                  >
                    All runs
                    <ArrowRight aria-hidden="true" className="size-3" />
                  </Link>
                }
              >
                <LoopRecentRuns runs={recentRuns} />
              </Section>
            </div>
            <aside className="flex flex-col gap-6">
              <LoopDeclaredInputs inputs={definition.inputs} />
              <LoopStartBindingsPanel
                declaredKinds={declaredKinds}
                bindings={bindings}
                jobs={bindingJobs}
                isLoading={bindingsLoading}
                onAddTrigger={onAddTrigger}
                onAddSchedule={onAddSchedule}
                triggers={bindingTriggers}
              />
              <LoopLimitsPanel effectiveConfig={effectiveConfig} />
              <LoopVersionsPanel version={loop.version} />
              {aggregate && successRate !== null ? (
                <LoopStatsPanel successRate={successRate} aggregate={aggregate} />
              ) : null}
            </aside>
          </div>
        </div>
      </div>
    </div>
  );
}
