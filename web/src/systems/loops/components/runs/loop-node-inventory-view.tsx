import { Link } from "@tanstack/react-router";
import { ChevronRight, Layers } from "lucide-react";

import {
  Button,
  CatalogCard,
  Empty,
  Eyebrow,
  ListingToolbar,
  type ListingViewMode,
  Pill,
  PillGroup,
  Section,
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@compozy/ui";

import {
  buildInventoryRow,
  LOOP_NODE_INVENTORY_LABELS,
  LOOP_NODE_INVENTORY_STATES,
  LOOP_NODE_INVENTORY_TONES,
  inventoryEmptyCopy,
  type LoopNodeInventoryRowView,
} from "../../lib/loop-node-inventory";
import { LOOP_STORY_ICONS } from "../../lib/loop-story-icons";
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

const STATE_ICON = {
  waiting: LOOP_STORY_ICONS.waiting,
  quarantined: LOOP_STORY_ICONS.quarantined,
  attention: LOOP_STORY_ICONS.attention,
  retrying: LOOP_STORY_ICONS.retry,
} as const;

const STATE_TONE_CLASS = {
  info: "text-info",
  danger: "text-danger",
  warning: "text-warning",
  accent: "text-accent",
  success: "text-success",
  neutral: "text-muted",
} as const;

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
  const empty = inventoryEmptyCopy(state, filtered);
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
      <p className="font-mono text-pill-group-badge text-faint">
        {`state=${state}`}
        {loopFilter === "" ? "" : ` · loop=${loopFilter}`}
        {runFilter === "" ? "" : ` · run_id=${runFilter}`}
      </p>
      {isLoading ? (
        <p className="px-1 text-small-body text-muted" data-testid="loop-node-inventory-loading">
          Loading {LOOP_NODE_INVENTORY_LABELS[state].toLowerCase()} nodes…
        </p>
      ) : rows.length === 0 ? (
        <Empty
          action={
            filtered ? (
              <Button onClick={onClearFilters} size="sm" type="button" variant="outline">
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
      ) : view === "cards" ? (
        <InventoryCards
          hasMore={hasMore}
          isFetchingNextPage={isFetchingNextPage}
          loadedCount={loadedCount}
          onLoadMore={onLoadMore}
          rows={rows}
        />
      ) : (
        <InventoryRows
          hasMore={hasMore}
          isFetchingNextPage={isFetchingNextPage}
          loadedCount={loadedCount}
          onLoadMore={onLoadMore}
          rows={rows}
        />
      )}
    </Section>
  );
}

function InventoryStateGlyph({ state }: { state: LoopNodeInventoryState }) {
  const Icon = STATE_ICON[state];
  const tone = LOOP_NODE_INVENTORY_TONES[state];
  return <Icon aria-hidden="true" className={`size-3.5 shrink-0 ${STATE_TONE_CLASS[tone]}`} />;
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
          variant="outline"
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
    <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
      <div className="flex items-center gap-4 border-b border-line px-4 py-2">
        <Eyebrow className="min-w-0 flex-1 text-muted">Step</Eyebrow>
        <Eyebrow className="hidden min-w-0 flex-1 text-muted min-[900px]:block">Loop · run</Eyebrow>
        <Eyebrow className="hidden min-w-0 flex-1 text-muted min-[720px]:block">Reason</Eyebrow>
        <Eyebrow className="shrink-0 text-right text-muted">Age</Eyebrow>
        <span aria-hidden="true" className="size-3.5 shrink-0" />
      </div>
      {rows.map((row, index) => (
        <Link
          className={`flex items-center gap-4 px-4 py-3 hover:bg-canvas-tint focus-visible:bg-canvas-tint ${
            index > 0 ? "border-t border-line-soft" : ""
          }`}
          data-testid={`loop-node-inventory-row-${row.key}`}
          key={row.key}
          params={{ runId: row.runId }}
          to="/loop-runs/$runId"
        >
          <span className="flex min-w-0 flex-1 items-start gap-2">
            <InventoryStateGlyph state={row.state} />
            <span className="min-w-0">
              <span className="block truncate text-ws-name font-medium text-fg-strong">
                {row.label}
              </span>
              <span className="mt-0.5 block truncate font-mono text-pill-group-badge text-faint">
                {row.micro}
              </span>
            </span>
          </span>
          <span className="hidden min-w-0 flex-1 truncate text-small-body text-muted min-[900px]:block">
            {row.loopName} <span className="font-mono text-mono-id text-faint">{row.runId}</span>
          </span>
          <span className="hidden min-w-0 flex-1 truncate text-small-body text-muted min-[720px]:block">
            {row.reason}
          </span>
          <span className="shrink-0 text-right">
            <span className="block font-mono text-mono-id tabular-nums text-subtle">{row.age}</span>
            <span className="block font-mono text-pill-group-badge text-faint">in this state</span>
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
                <InventoryStateGlyph state={row.state} />
                <span className="min-w-0 flex-1 truncate text-ws-name font-medium text-fg-strong">
                  {row.label}
                </span>
                <Pill size="xs" tone={LOOP_NODE_INVENTORY_TONES[row.state]}>
                  {LOOP_NODE_INVENTORY_LABELS[row.state]}
                </Pill>
              </span>
              <CatalogCard.Description className="line-clamp-2">
                {row.reason}
              </CatalogCard.Description>
              <span className="font-mono text-mono-id text-faint">
                {row.micro} · {row.age} in this state
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
