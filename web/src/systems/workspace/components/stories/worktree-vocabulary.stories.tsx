import type { Meta, StoryObj } from "@storybook/react-vite";

import { Eyebrow } from "@compozy/ui";

import { CenteredSurface } from "@/storybook/story-layout";
import {
  LONG_WORKTREE_PATH,
  discoveredWorktreeFixture,
  worktreeBehindFixture,
  worktreeErrorFixture,
  worktreeMergedFixture,
  worktreeMissingFixture,
  worktreePendingFixture,
  worktreeReadyDirtyRunningFixture,
  worktreeSetupFlaggedFixture,
} from "@/systems/workspace/mocks";

import { toWorktreeNestEntries } from "../../lib/worktree-display";
import { WorktreeRow } from "../worktree-row";
import { WorktreeSignals } from "../worktree-signals";
import { WorktreeStateChip } from "../worktree-state-chip";
import { WorktreeStateDot } from "../worktree-state-dot";

const CHIP_STATES = ["pending", "discovered", "missing", "error", "failed"] as const;
const DOT_STATES = CHIP_STATES;

function Sheet({ children }: { children: React.ReactNode }) {
  return (
    <CenteredSurface>
      <div className="w-full max-w-3xl">{children}</div>
    </CenteredSurface>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="flex items-center gap-4 border-b border-line-soft py-3 last:border-b-0">
      <Eyebrow className="w-48 shrink-0 text-faint">{label}</Eyebrow>
      <div className="flex min-w-0 items-center gap-3">{children}</div>
    </div>
  );
}

const meta: Meta<typeof WorktreeRow> = {
  title: "systems/workspace/components/WorktreeVocabulary",
  component: WorktreeRow,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "The shared worktree row vocabulary: state chips, nested state dots, companion signals, and the composed row at both densities.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * VC-01 — the state chip inventory. Ready carries no chip at all.
 * `failed` is the N-004 lifecycle chip (authorized delta vs the five-state artboard).
 */
export const StateChips: Story = {
  args: {},
  render: () => (
    <Sheet>
      <Row label="ready">
        <span className="text-form-hint text-subtle">
          no chip — the quiet success dot is the whole signal
        </span>
      </Row>
      {CHIP_STATES.map(state => (
        <Row key={state} label={state}>
          <WorktreeStateChip state={state} />
        </Row>
      ))}
    </Sheet>
  ),
};

/** VC-02 — companion signals; unknown facts render absent, never as zero. */
export const Signals: Story = {
  args: {},
  render: () => (
    <Sheet>
      <Row label="dirty · counts known">
        <WorktreeSignals
          dirty
          insertions={142}
          deletions={18}
          changedFiles={6}
          ahead={null}
          behind={null}
          agentActivity="idle"
          origin="manual"
          setupState="ok"
        />
      </Row>
      <Row label="dirty · magnitude unknown">
        <WorktreeSignals
          dirty
          ahead={null}
          behind={null}
          agentActivity="idle"
          origin="manual"
          setupState="ok"
        />
      </Row>
      <Row label="ahead / behind">
        <WorktreeSignals
          dirty={null}
          ahead={3}
          behind={2}
          agentActivity="idle"
          origin="manual"
          setupState="ok"
        />
      </Row>
      <Row label="agent running">
        <WorktreeSignals
          dirty={null}
          ahead={null}
          behind={null}
          agentActivity="running"
          origin="manual"
          setupState="ok"
        />
      </Row>
      <Row label="agent awaiting input">
        <WorktreeSignals
          dirty={null}
          ahead={null}
          behind={null}
          agentActivity="awaiting_input"
          origin="manual"
          setupState="ok"
        />
      </Row>
      <Row label="agent idle">
        <span className="text-form-hint text-subtle">renders nothing in rows</span>
      </Row>
      <Row label="origin · run-created">
        <WorktreeSignals
          dirty={null}
          ahead={null}
          behind={null}
          agentActivity="idle"
          origin="per_run"
          setupState="ok"
        />
      </Row>
      <Row label="setup flagged">
        <WorktreeSignals
          dirty={null}
          ahead={null}
          behind={null}
          agentActivity="idle"
          origin="manual"
          setupState="failed"
          setupError="bun install exited 1"
        />
      </Row>
      <Row label="merged">
        <WorktreeSignals
          dirty={null}
          ahead={null}
          behind={null}
          agentActivity="idle"
          origin="manual"
          setupState="ok"
          merged
        />
      </Row>
      <Row label="stale remote value">
        <WorktreeSignals
          dirty={null}
          ahead={1}
          behind={null}
          agentActivity="idle"
          origin="manual"
          setupState="ok"
          staleLabel="18m ago"
        />
      </Row>
      <Row label="unknown (no upstream)">
        <span className="font-mono text-mono-id text-faint">no upstream</span>
      </Row>
    </Sheet>
  ),
};

/**
 * VC-03 — the busiest realistic list row: ready, dirty, and running.
 * Full-row budget is two (agent → dirty → ahead-behind → merged), so ahead drops.
 * List payloads have no insertion/deletion counts, so dirty reads "uncommitted".
 * activityAt sorts the nest; the row does not paint a relative clock
 * (authorized delta vs artboard §03 — see task-06-authorized-deltas.md).
 */
export const RowReadyDirtyRunning: Story = {
  args: {},
  render: () => {
    const [entry] = toWorktreeNestEntries({
      worktrees: [worktreeReadyDirtyRunningFixture],
      discovered: [],
    });
    return <Sheet>{entry ? <WorktreeRow entry={entry} /> : null}</Sheet>;
  },
};

/**
 * VC-04 — discovered, pending, missing, and error rows side by side.
 * Error is inert with the functional reason "status read failed".
 */
export const RowStates: Story = {
  args: {},
  render: () => {
    const entries = toWorktreeNestEntries({
      worktrees: [
        worktreeBehindFixture,
        worktreeMergedFixture,
        worktreeSetupFlaggedFixture,
        worktreePendingFixture,
        worktreeMissingFixture,
        worktreeErrorFixture,
      ],
      discovered: [discoveredWorktreeFixture],
    });
    return (
      <Sheet>
        {entries.map(entry => (
          <WorktreeRow
            key={entry.key}
            entry={entry}
            merged={entry.name === worktreeMergedFixture.name}
            trailing={
              entry.adoptable ? (
                <span className="text-mono-id font-semibold text-info">Adopt</span>
              ) : null
            }
          />
        ))}
      </Sheet>
    );
  },
};

/**
 * VC-05 — nested state dots plus a full detached row ("pinned at <sha>").
 * `failed` stays in the inventory (N-004). Production nests use chip + folder
 * well, not the dot — the sheet still captures the shape source of truth.
 */
export const NestDotsAndDetached: Story = {
  args: {},
  render: () => {
    const [nested] = toWorktreeNestEntries({
      worktrees: [{ ...worktreeReadyDirtyRunningFixture, path: LONG_WORKTREE_PATH }],
      discovered: [],
    });
    const [detached] = toWorktreeNestEntries({
      worktrees: [worktreeBehindFixture],
      discovered: [],
    });
    return (
      <Sheet>
        {DOT_STATES.map(state => (
          <Row key={state} label={`dot · ${state}`}>
            <WorktreeStateDot state={state} />
            <WorktreeStateChip state={state} />
          </Row>
        ))}
        {detached ? <WorktreeRow entry={detached} detachedSha="9ec3d15aa1b2" /> : null}
        <Row label="nested density">
          <div className="w-96">
            {nested ? <WorktreeRow entry={nested} density="nest" /> : null}
          </div>
        </Row>
      </Sheet>
    );
  },
};
