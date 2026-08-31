import type { Meta, StoryObj } from "@storybook/react-vite";

import { LoopNodeInventoryView } from "../runs/loop-node-inventory-view";
import { LoopRunsView } from "../runs/loop-runs-view";
import type { LoopNodeInventoryItem, LoopNodeInventoryState, LoopRun } from "../../types";
import { loopRunFixtures } from "../../mocks/fixtures";
import { STORY_NOW } from "./loop-run-page-fixture-world";
import { scopedListingScopeFixture } from "@/systems/profiles/mocks";

/**
 * Visual Contract capture target for the workspace node inventories (VC-R5).
 *
 * The route returns `{items, next_cursor}` and publishes no totals, so these
 * stories deliberately carry no count badges: the foot reports what is loaded,
 * and "Load more" appears only while the server still holds a cursor.
 */

function inventoryItem(
  state: LoopNodeInventoryState,
  overrides: Partial<LoopNodeInventoryItem>
): LoopNodeInventoryItem {
  return {
    state,
    loop_run_id: "r-914c3d",
    loop_name: "issue-triage",
    generation: 4,
    node_id: "triage",
    item_index: 2,
    state_at: new Date(STORY_NOW - 49 * 60_000).toISOString(),
    ...overrides,
  };
}

const WAITING_ITEMS: LoopNodeInventoryItem[] = [
  inventoryItem("waiting", {
    node_id: "approve",
    item_index: 2,
    wait: {
      loop_run_id: "r-914c3d",
      generation: 4,
      node_id: "approve",
      item_index: 2,
      kind: "approval_escalation",
      escalation_cursor: 1,
      claim_state: "intervention_required",
      admission_failures: 0,
      issued_epoch: 7,
      created_at: new Date(STORY_NOW - 49 * 60_000).toISOString(),
      age_seconds: 49 * 60,
    },
  }),
  inventoryItem("waiting", {
    loop_run_id: "r-2b81aa",
    loop_name: "docs-refresh",
    node_id: "publish",
    item_index: 0,
    generation: 1,
    state_at: new Date(STORY_NOW - 6 * 60_000).toISOString(),
    wait: {
      loop_run_id: "r-2b81aa",
      generation: 1,
      node_id: "publish",
      item_index: 0,
      kind: "timer",
      escalation_cursor: 0,
      claim_state: "waiting",
      admission_failures: 0,
      issued_epoch: 2,
      created_at: new Date(STORY_NOW - 6 * 60_000).toISOString(),
      age_seconds: 6 * 60,
      resume_at: new Date(STORY_NOW + 18 * 60_000).toISOString(),
    },
  }),
];

function InventoryStoryHost({
  state,
  items,
  hasMore = false,
  loopFilter = "",
  view = "rows",
}: {
  state: LoopNodeInventoryState;
  items: LoopNodeInventoryItem[];
  hasMore?: boolean;
  loopFilter?: string;
  view?: "rows" | "cards";
}) {
  return (
    <div className="min-h-dvh bg-canvas p-6">
      <LoopNodeInventoryView
        hasMore={hasMore}
        isFetchingNextPage={false}
        isLoading={false}
        items={items}
        loadedCount={items.length}
        loopFilter={loopFilter}
        loopOptions={["issue-triage", "docs-refresh", "implement-tasks"]}
        nowMs={STORY_NOW}
        onClearFilters={() => {}}
        onLoadMore={() => {}}
        onLoopFilterChange={() => {}}
        onRunFilterChange={() => {}}
        onStateChange={() => {}}
        onViewChange={() => {}}
        runFilter=""
        runOptions={["r-914c3d", "r-2b81aa"]}
        state={state}
        view={view}
      />
    </div>
  );
}

const meta: Meta<typeof LoopNodeInventoryView> = {
  title: "systems/loops/components/LoopNodeInventory",
  component: LoopNodeInventoryView,
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

export const Waiting: Story = {
  args: {},
  render: () => <InventoryStoryHost items={WAITING_ITEMS} state="waiting" />,
};

/** Cards view of the same waiting inventory — state glyph, one pill, clamped reason. */
export const WaitingCards: Story = {
  args: {},
  render: () => <InventoryStoryHost items={WAITING_ITEMS} state="waiting" view="cards" />,
};

export const WaitingWithMore: Story = {
  args: {},
  render: () => <InventoryStoryHost hasMore items={WAITING_ITEMS} state="waiting" />,
};

export const Quarantined: Story = {
  args: {},
  render: () => (
    <InventoryStoryHost
      items={[
        inventoryItem("quarantined", {
          node_id: "fix_batch",
          control: {
            loop_run_id: "r-914c3d",
            node_id: "fix_batch",
            paused: false,
            quarantined: true,
            death_resume_streak: 0,
            revision: 4,
            updated_at: new Date(STORY_NOW - 9 * 60_000).toISOString(),
            quarantine_entry: {
              node_id: "fix_batch",
              input_ref: ".compozy/tasks/loops-paper/task_03.md",
              target: "toolcall:github-mcp",
              episodes: [{ generation: 2, attempts: [{ Attempt: 1 }, { Attempt: 2 }] }],
            },
          },
        }),
      ]}
      state="quarantined"
    />
  ),
};

export const Attention: Story = {
  args: {},
  render: () => (
    <InventoryStoryHost
      items={[
        inventoryItem("attention", {
          node_id: "collect_fixes",
          control: {
            loop_run_id: "r-914c3d",
            node_id: "collect_fixes",
            paused: false,
            quarantined: false,
            death_resume_streak: 0,
            revision: 2,
            updated_at: new Date(STORY_NOW - 2 * 60_000).toISOString(),
            attention_flag: "dependency_quarantined",
            attention_reason: "The join needs the output of fix_batch, which is quarantined.",
          },
        }),
      ]}
      state="attention"
    />
  ),
};

export const Retrying: Story = {
  args: {},
  render: () => (
    <InventoryStoryHost
      items={[
        inventoryItem("retrying", {
          node_id: "execute_task",
          output: {
            node_id: "execute_task",
            status: "failed",
            attempt: 2,
            failure_class: "transport",
            next_attempt_at: new Date(STORY_NOW + 38_000).toISOString(),
          },
        }),
      ]}
      state="retrying"
    />
  ),
};

/** The truthful empty state — no filters applied. */
export const Empty: Story = {
  args: {},
  render: () => <InventoryStoryHost items={[]} state="attention" />,
};

/** The filter-specific empty state, which never claims the workspace is clear. */
export const EmptyFiltered: Story = {
  args: {},
  render: () => <InventoryStoryHost items={[]} loopFilter="docs-refresh" state="waiting" />,
};

/**
 * The runs-area delta this spec introduces: `canceled` joins the roster and the
 * outcome filter as a neutral terminal — a deliberate ending, not a failure.
 *
 * The view is filtered to `canceled` so the rows and their pills are actually in
 * frame: a canceled run is terminal, so under the `all` filter it sorts into the
 * Past table beneath every active run and falls below the fold.
 */
const CANCELED_RUNS: LoopRun[] = [
  {
    ...loopRunFixtures[0],
    id: "looprun_canceled",
    status: "canceled",
    loop_name: "implement-tasks",
  },
  {
    ...loopRunFixtures[1],
    id: "looprun_canceled_2",
    status: "canceled",
    loop_name: "review-and-fix",
  },
];

export const RunsRosterCanceled: Story = {
  args: {},
  render: () => (
    <div className="min-h-dvh bg-canvas p-6">
      <LoopRunsView
        profileScope={scopedListingScopeFixture}
        outcome="canceled"
        runs={[...loopRunFixtures, ...CANCELED_RUNS]}
      />
    </div>
  ),
};
