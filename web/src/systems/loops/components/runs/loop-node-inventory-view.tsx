import { Link } from "@tanstack/react-router";
import { ChevronRight, Layers } from "lucide-react";

import {
  Button,
  CatalogCard,
  Empty,
  Eyebrow,
  ListingToolbar,
  type ListingViewMode,
  StateGlyph,
  PillGroup,
  Section,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
  SkeletonRows,
} from "@compozy/ui";

import {
  buildInventoryRow,
  LOOP_NODE_INVENTORY_LABELS,
  LOOP_NODE_INVENTORY_STATES,
  LOOP_NODE_INVENTORY_GLYPHS,
  inventoryEmptyCopy,
  type LoopNodeInventoryRowView,
} from "../../lib/loop-node-inventory";
import type { LoopNodeInventoryItem, LoopNodeInventoryState } from "../../types";

export interface LoopNodeInventoryViewProps {
  state: LoopNodeInventoryState;
  items: readonly LoopNodeInventoryItem[];
  /** Rows currently loaded. Never a population total — the route publishes none. */
  loadedCount: number;
  hasMore: boolean;
  isLoading: boolean;
  isFetchingNextPage: boolean;
  loopFilter: string;
  runFilter: string;
  view: ListingViewMode;
  /** Loop names offered by the filter, from the workspace catalog. */
  loopOptions: readonly string[];
  /** Run ids offered by the filter, from the loaded runs page. */
  runOptions: readonly string[];
  /** Clock used for age derivation, so fixtures and stories stay deterministic. */
  nowMs: number;
  onStateChange: (state: LoopNodeInventoryState) => void;
  onLoopFilterChange: (loop: string) => void;
  onRunFilterChange: (runId: string) => void;
  onViewChange: (view: ListingViewMode) => void;
  onClearFilters: () => void;
  onLoadMore: () => void;
}

const ALL_LOOPS = "__all__";
const ALL_RUNS = "__all_runs__";

/**
 * The workspace node inventory (VC-R5): one list, four state filters, the same
 * route with a different `?state=`.
 *
 * Truthfulness note — the route returns `{items, next_cursor}` and publishes no
 * totals, so the state tabs carry no count badges and the foot says how many
 * rows are *loaded*, never how many exist. Presenting a loaded page as a
 * population is the completeness lie the catalog contract forbids (SD-007).
 */
export function LoopNodeInventoryView({
  state,
  items,
  loadedCount,
  hasMore,
  isLoading,
  isFetchingNextPage,
  loopFilter,
  runFilter,
  view,
  loopOptions,
  runOptions,
  nowMs,
  onStateChange,
  onLoopFilterChange,
  onRunFilterChange,
  onViewChange,
  onClearFilters,
  onLoadMore,
}: LoopNodeInventoryViewProps) {
  const rows = items.map(item => buildInventoryRow(item, nowMs));
  const filtered = loopFilter !== "" || runFilter !== "";
  return (
    <Section
      data-testid="loop-node-inventory"
      icon={Layers}
      label="Inventory"
      bodyClassName="gap-4"
    >
      <ListingToolbar>
        <ListingToolbar.Leading>
          {/* No count badges: the route publishes `{items, next_cursor}` and no
              totals, so a per-state number here would be invented (SD-007). */}
          <PillGroup
            aria-label="Step inventory state"
            items={LOOP_NODE_INVENTORY_STATES.map(value => ({
              value,
              label: LOOP_NODE_INVENTORY_LABELS[value],
              testId: `loop-node-inventory-state-${value}`,
            }))}
            onChange={onStateChange}
            value={state}
          />
          <InventoryLoopFilter
            loopFilter={loopFilter}
            loopOptions={loopOptions}
            onLoopFilterChange={onLoopFilterChange}
          />
          <InventoryRunFilter
            onRunFilterChange={onRunFilterChange}
            runFilter={runFilter}
            runOptions={runOptions}
          />
          {filtered ? (
            <Button
              data-testid="loop-node-inventory-clear-filters"
              onClick={onClearFilters}
              size="sm"
              type="button"
              variant="ghost"
            >
              Clear filters
            </Button>
          ) : null}
        </ListingToolbar.Leading>
        <ListingToolbar.Trailing>
          <ListingToolbar.ViewToggle onChange={onViewChange} value={view} />
        </ListingToolbar.Trailing>
      </ListingToolbar>
      <InventoryResults
        filtered={filtered}
        hasMore={hasMore}
        isFetchingNextPage={isFetchingNextPage}
        isLoading={isLoading}
        loadedCount={loadedCount}
        onClearFilters={onClearFilters}
        onLoadMore={onLoadMore}
        rows={rows}
        state={state}
        view={view}
      />
    </Section>
  );
}

function InventoryLoopFilter({
  loopFilter,
  loopOptions,
  onLoopFilterChange,
}: Pick<LoopNodeInventoryViewProps, "loopFilter" | "loopOptions" | "onLoopFilterChange">) {
  return (
    <Select
      onValueChange={value => onLoopFilterChange(value === ALL_LOOPS ? "" : (value ?? ""))}
      value={loopFilter === "" ? ALL_LOOPS : loopFilter}
    >
      <SelectTrigger className="w-48" data-testid="loop-node-inventory-loop-filter" size="sm">
        <SelectValue>{loopFilter === "" ? "All loops" : loopFilter}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_LOOPS}>All loops</SelectItem>
        {loopOptions.map(name => (
          <SelectItem key={name} value={name}>
            {name}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

function InventoryRunFilter({
  runFilter,
  runOptions,
  onRunFilterChange,
}: Pick<LoopNodeInventoryViewProps, "runFilter" | "runOptions" | "onRunFilterChange">) {
  return (
    <Select
      onValueChange={value => onRunFilterChange(value === ALL_RUNS ? "" : (value ?? ""))}
      value={runFilter === "" ? ALL_RUNS : runFilter}
    >
      <SelectTrigger className="w-48" data-testid="loop-node-inventory-run-filter" size="sm">
        <SelectValue>{runFilter === "" ? "All runs" : runFilter}</SelectValue>
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ALL_RUNS}>All runs</SelectItem>
        {runOptions.map(id => (
          <SelectItem key={id} value={id}>
            {id}
          </SelectItem>
        ))}
        {runFilter !== "" && !runOptions.includes(runFilter) ? (
          <SelectItem value={runFilter}>{runFilter}</SelectItem>
        ) : null}
      </SelectContent>
    </Select>
  );
}

function InventoryResults({
  state,
  rows,
  view,
  filtered,
  isLoading,
  onClearFilters,
  ...page
}: {
  state: LoopNodeInventoryState;
  rows: readonly LoopNodeInventoryRowView[];
  view: ListingViewMode;
  filtered: boolean;
  isLoading: boolean;
  hasMore: boolean;
  isFetchingNextPage: boolean;
  loadedCount: number;
  onClearFilters: () => void;
  onLoadMore: () => void;
}) {
  if (isLoading) {
    return (
      <div
        aria-busy="true"
        aria-label={`Loading ${LOOP_NODE_INVENTORY_LABELS[state].toLowerCase()} steps`}
        className="rounded-lg bg-canvas shadow-card p-4"
        data-testid="loop-node-inventory-loading"
      >
        <SkeletonRows className="gap-4" count={4} />
      </div>
    );
  }
  if (rows.length === 0) {
    const empty = inventoryEmptyCopy(state, filtered);
    return (
      <Empty
        action={
          filtered ? (
            <Button onClick={onClearFilters} size="sm" type="button" variant="secondary">
              Clear filters
            </Button>
          ) : undefined
        }
        className="mx-auto my-8 max-w-md"
        data-testid="loop-node-inventory-empty"
        description={empty.description}
        icon={empty.icon}
        title={empty.title}
      />
    );
  }
  if (view === "cards") return <InventoryCards rows={rows} {...page} />;
  return <InventoryRows rows={rows} {...page} />;
}

function InventoryFoot({
  hasMore,
  isFetchingNextPage = false,
  loadedCount,
  onLoadMore,
}: {
  hasMore: boolean;
  isFetchingNextPage?: boolean;
  loadedCount: number;
  onLoadMore: () => void;
}) {
  return (
    <div className="flex items-center justify-between gap-3 border-t border-line-soft px-4 py-2.5">
      <span
        className="font-mono text-pill-group-badge text-faint"
        data-testid="loop-node-inventory-foot"
      >
        {hasMore ? `Showing ${loadedCount} loaded · more available` : `Showing all ${loadedCount}`}
      </span>
      {hasMore ? (
        <Button
          data-testid="loop-node-inventory-load-more"
          disabled={isFetchingNextPage}
          onClick={onLoadMore}
          size="sm"
          type="button"
          variant="secondary"
        >
          {isFetchingNextPage ? "Loading…" : "Load more"}
        </Button>
      ) : null}
    </div>
  );
}

function InventoryRows({
  rows,
  hasMore,
  isFetchingNextPage,
  loadedCount,
  onLoadMore,
}: {
  rows: readonly LoopNodeInventoryRowView[];
  hasMore: boolean;
  isFetchingNextPage: boolean;
  loadedCount: number;
  onLoadMore: () => void;
}) {
  return (
    <div className="overflow-hidden rounded-lg bg-canvas shadow-card">
      <div className="flex items-center gap-4 border-b border-line px-4 py-2">
        <Eyebrow className="min-w-0 flex-1 text-muted">Step</Eyebrow>
        <Eyebrow className="hidden min-w-0 flex-1 text-muted lg:block">Loop</Eyebrow>
        <Eyebrow className="hidden min-w-0 flex-1 text-muted md:block">Reason</Eyebrow>
        <Eyebrow className="w-24 shrink-0 text-right text-muted">Time in state</Eyebrow>
        <span aria-hidden="true" className="size-3.5 shrink-0" />
      </div>
      {rows.map((row, index) => (
        <Link
          className={`flex items-center gap-4 px-4 py-3 hover:bg-surface-2 focus-visible:bg-surface-2 ${
            index > 0 ? "border-t border-line-soft" : ""
          }`}
          data-testid={`loop-node-inventory-row-${row.key}`}
          key={row.key}
          params={{ runId: row.runId }}
          to="/loop-runs/$runId"
        >
          <span className="flex min-w-0 flex-1 items-start gap-2">
            <StateGlyph className="mt-0.5" state={LOOP_NODE_INVENTORY_GLYPHS[row.state]} />
            <span className="min-w-0">
              <span
                className="block truncate text-ws-name font-medium text-fg-strong"
                title={row.nodeId}
              >
                {row.label}
              </span>
              <span className="mt-0.5 block truncate text-small-body text-faint">{row.micro}</span>
            </span>
          </span>
          <span
            className="hidden min-w-0 flex-1 truncate text-small-body text-muted lg:block"
            title={row.runId}
          >
            {row.loopName}
          </span>
          <span className="hidden min-w-0 flex-1 truncate text-small-body text-muted md:block">
            {row.reason}
          </span>
          <span className="w-24 shrink-0 text-right">
            <span className="block font-mono text-mono-id tabular-nums text-subtle">{row.age}</span>
          </span>
          <ChevronRight aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
        </Link>
      ))}
      <InventoryFoot
        hasMore={hasMore}
        isFetchingNextPage={isFetchingNextPage}
        loadedCount={loadedCount}
        onLoadMore={onLoadMore}
      />
    </div>
  );
}

function InventoryCards({
  rows,
  hasMore,
  isFetchingNextPage,
  loadedCount,
  onLoadMore,
}: {
  rows: readonly LoopNodeInventoryRowView[];
  hasMore: boolean;
  isFetchingNextPage: boolean;
  loadedCount: number;
  onLoadMore: () => void;
}) {
  return (
    <div>
      <div
        className="grid grid-cols-[repeat(auto-fill,minmax(250px,1fr))] gap-3"
        data-testid="loop-node-inventory-card-grid"
      >
        {rows.map(row => (
          <CatalogCard actionable data-testid={`loop-node-inventory-card-${row.key}`} key={row.key}>
            <Link
              className="flex min-w-0 flex-col gap-3"
              params={{ runId: row.runId }}
              to="/loop-runs/$runId"
            >
              <span className="flex items-center gap-2">
                <StateGlyph className="mt-0.5" state={LOOP_NODE_INVENTORY_GLYPHS[row.state]} />
                <span className="min-w-0 flex-1 truncate text-ws-name font-medium text-fg-strong">
                  {row.label}
                </span>
                <span className="shrink-0 text-eyebrow text-fg-2">
                  {LOOP_NODE_INVENTORY_LABELS[row.state]}
                </span>
              </span>
              <CatalogCard.Description className="line-clamp-2">
                {row.reason}
              </CatalogCard.Description>
              <span className="text-small-body text-faint">
                {row.micro} · <span className="font-mono text-mono-id tabular-nums">{row.age}</span>
              </span>
            </Link>
          </CatalogCard>
        ))}
      </div>
      <InventoryFoot
        hasMore={hasMore}
        isFetchingNextPage={isFetchingNextPage}
        loadedCount={loadedCount}
        onLoadMore={onLoadMore}
      />
    </div>
  );
}
