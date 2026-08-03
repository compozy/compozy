import { Link } from "@tanstack/react-router";
import { ChevronRight, Layers } from "lucide-react";

import {
  Button,
  Empty,
  Eyebrow,
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
  type LoopNodeInventorySort,
  inventoryEmptyCopy,
  sortInventoryRows,
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
  sort: LoopNodeInventorySort;
  loopFilter: string;
  runFilter: string;
  /** Loop names offered by the filter, from the workspace catalog. */
  loopOptions: readonly string[];
  /** Clock used for age derivation, so fixtures and stories stay deterministic. */
  nowMs: number;
  onStateChange: (state: LoopNodeInventoryState) => void;
  onSortChange: (sort: LoopNodeInventorySort) => void;
  onLoopFilterChange: (loop: string) => void;
  onClearFilters: () => void;
  onLoadMore: () => void;
}

const ALL_LOOPS = "__all__";

const SORT_LABEL: Record<LoopNodeInventorySort, string> = {
  oldest: "Longest in state",
  newest: "Newest first",
};

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
  sort,
  loopFilter,
  runFilter,
  loopOptions,
  nowMs,
  onStateChange,
  onSortChange,
  onLoopFilterChange,
  onClearFilters,
  onLoadMore,
}: LoopNodeInventoryViewProps) {
  const rows = sortInventoryRows(
    items.map(item => buildInventoryRow(item, nowMs)),
    sort
  );
  const filtered = loopFilter !== "" || runFilter !== "";
  const empty = inventoryEmptyCopy(state, filtered);
  return (
    <Section
      data-testid="loop-node-inventory"
      icon={Layers}
      label="Inventory"
      note="One list, four state filters — the same route with a different state."
      bodyClassName="gap-4"
    >
      <div className="flex flex-wrap items-center gap-3">
        {/* No count badges: the route publishes `{items, next_cursor}` and no
            totals, so a per-state number here would be invented (SD-007). */}
        <PillGroup
          aria-label="Node inventory state"
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
            {/* The sentinel is an internal value, never operator-facing copy. */}
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
          onValueChange={value => onSortChange((value ?? "oldest") as LoopNodeInventorySort)}
          value={sort}
        >
          <SelectTrigger className="w-44" data-testid="loop-node-inventory-sort" size="sm">
            <SelectValue>{SORT_LABEL[sort]}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="oldest">{SORT_LABEL.oldest}</SelectItem>
            <SelectItem value="newest">{SORT_LABEL.newest}</SelectItem>
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
      </div>
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
          icon={Layers}
          title={empty.title}
        />
      ) : (
        <div className="overflow-hidden rounded-lg border border-line bg-canvas-soft">
          <div className="flex items-center gap-4 border-b border-line px-4 py-2">
            <Eyebrow className="min-w-0 flex-1 text-muted">Node</Eyebrow>
            <Eyebrow className="hidden min-w-0 flex-1 text-muted min-[900px]:block">
              Loop · run
            </Eyebrow>
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
              <span className="min-w-0 flex-1">
                <span className="block truncate text-ws-name font-medium text-fg-strong">
                  {row.label}
                </span>
                <span className="mt-0.5 block truncate font-mono text-pill-group-badge text-faint">
                  {row.micro}
                </span>
              </span>
              <span className="hidden min-w-0 flex-1 truncate text-small-body text-muted min-[900px]:block">
                {row.loopName}{" "}
                <span className="font-mono text-mono-id text-faint">{row.runId}</span>
              </span>
              <span className="hidden min-w-0 flex-1 truncate text-small-body text-muted min-[720px]:block">
                {row.reason}
              </span>
              <span className="shrink-0 text-right">
                <span className="block font-mono text-mono-id tabular-nums text-subtle">
                  {row.age}
                </span>
                <span className="block font-mono text-pill-group-badge text-faint">
                  in this state
                </span>
              </span>
              <ChevronRight aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
            </Link>
          ))}
          <div className="flex items-center justify-between gap-3 border-t border-line-soft px-4 py-2.5">
            <span
              className="font-mono text-pill-group-badge text-faint"
              data-testid="loop-node-inventory-foot"
            >
              {hasMore
                ? `Showing ${loadedCount} loaded · more available`
                : `Showing all ${loadedCount}`}
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
        </div>
      )}
    </Section>
  );
}
