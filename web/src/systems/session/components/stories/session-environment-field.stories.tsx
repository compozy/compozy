import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { fn, userEvent, within } from "storybook/test";

import type { NetworkParticipationDraft } from "@/lib/network-participation";
import { StorySurface } from "@/storybook/story-layout";
import { agentFixtures } from "@/systems/agent/mocks";
import type { WorktreeMaterialization, WorktreePayload } from "@/systems/workspace";
import {
  workspaceDetailFixture,
  worktreeBehindFixture,
  worktreeMergedFixture,
  worktreeMissingFixture,
  worktreeReadyDirtyRunningFixture,
} from "@/systems/workspace/mocks";

import type { SessionEnvironmentTarget } from "../../lib/session-environment-target";
import { SessionCreateDialog } from "../session-create-dialog";
import { SessionEnvironmentField } from "../session-environment-field";

const WORKTREES = [worktreeReadyDirtyRunningFixture, worktreeBehindFixture, worktreeMergedFixture];
const MISSING_SELECTION = {
  kind: "worktree" as const,
  worktreeId: worktreeMissingFixture.id,
};
const NEW_TARGET: SessionEnvironmentTarget = {
  kind: "new",
  name: "docs-refresh",
  previous: { kind: "root" },
};

/** A materialization with inert controls: stories state a phase, they never drive one. */
function materialization(
  overrides: Partial<WorktreeMaterialization> = {}
): WorktreeMaterialization {
  return {
    status: "idle",
    start: fn(),
    cancel: fn(),
    recover: fn(),
    retry: fn(),
    reset: fn(),
    ...overrides,
  };
}

/** Every production-owned creation status. `phase` exists only while pending. */
const CREATION_PHASES: WorktreeMaterialization[] = [
  materialization({ status: "creating" }),
  materialization({ status: "pending", phase: "branch" }),
  materialization({ status: "pending", phase: "checkout" }),
  materialization({ status: "pending", phase: "copy" }),
  materialization({ status: "pending", phase: "setup" }),
];

function FieldRow({
  initial = { kind: "root" },
  worktrees = WORKTREES,
  materialization: live,
}: {
  initial?: SessionEnvironmentTarget;
  worktrees?: readonly WorktreePayload[];
  materialization?: WorktreeMaterialization;
}) {
  const [value, setValue] = useState<SessionEnvironmentTarget>(initial);
  return (
    <SessionEnvironmentField
      materialization={live}
      onChange={setValue}
      value={value}
      worktrees={worktrees}
    />
  );
}

function Harness(props: {
  initial?: SessionEnvironmentTarget;
  worktrees?: readonly WorktreePayload[];
  materialization?: WorktreeMaterialization;
}) {
  return (
    <StorySurface className="w-[420px]">
      <FieldRow {...props} />
    </StorySurface>
  );
}

const meta: Meta<typeof SessionEnvironmentField> = {
  title: "systems/session/components/SessionEnvironmentField",
  component: SessionEnvironmentField,
  parameters: {
    layout: "fullscreen",
    docs: {
      description: {
        component:
          "Where a new session will run. The control is absent — never disabled — on a workspace git cannot back, and a selection that stops being ready blocks submission instead of silently falling back to the root.",
      },
    },
  },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** VC-01 — the default: the workspace root, named by the locked vocabulary. */
export const Default: Story = {
  args: { value: { kind: "root" }, worktrees: WORKTREES, onChange: fn() },
  render: () => <Harness />,
};

/** VC-02 — open: only ready worktrees are offers, and "New worktree…" closes the list. */
export const OpenWithReadyWorktrees: Story = {
  args: Default.args,
  render: () => <Harness />,
  tags: ["play-fn"],
  play: async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    await userEvent.click(await body.findByTestId("session-create-environment"));
  },
};

/** VC-03 — a git workspace with nothing created yet: the root and the new path only. */
export const EmptyWorkspace: Story = {
  args: { ...Default.args, worktrees: [] },
  render: () => <Harness worktrees={[]} />,
  tags: ["play-fn"],
  play: OpenWithReadyWorktrees.play,
};

/** VC-04 — a worktree is selected, and the trigger carries its record label. */
export const WorktreeSelected: Story = {
  args: { ...Default.args, value: { kind: "worktree", worktreeId: worktreeBehindFixture.id } },
  render: () => <Harness initial={{ kind: "worktree", worktreeId: worktreeBehindFixture.id }} />,
};

/**
 * VC-05 — a workspace git cannot back has no environment to choose, so the
 * dialog carries no field at all rather than a disabled one.
 *
 * Production hides the field when `listing.repo.git_backed === false`, which
 * resolves to `environment` omitted and `environmentListingState="unsupported"`.
 * `WorkspacePayload` has no `git_backed` flag; the listing state is the dialog API.
 */
export const NonGitWorkspaceFieldAbsent: Story = {
  args: Default.args,
  parameters: { layout: "fullscreen" },
  render: () => {
    const workspace = workspaceDetailFixture.workspace;
    return (
      <SessionCreateDialog
        agents={agentFixtures}
        destinationLabel={workspace.name}
        destinationReady
        environmentListingState="unsupported"
        firstMessage=""
        isAwaitingEnvironment={false}
        isSubmitting={false}
        mode="advanced"
        onCancelEnvironment={fn()}
        onFirstMessageChange={fn()}
        networkParticipation={
          { mode: "local", channelStrategy: "", channelId: "" } satisfies NetworkParticipationDraft
        }
        onAgentChange={fn()}
        onModeChange={fn()}
        onNetworkParticipationChange={fn()}
        onOpenChange={fn()}
        onSessionNameChange={fn()}
        onSubmit={fn()}
        open
        restoreFocusOnClose
        scope="workspace"
        selectedAgentName={agentFixtures[0]?.name ?? ""}
        sessionName="Investigate checkout latency"
        sessionRoot={workspace.root_dir}
        submitError={null}
      />
    );
  },
};

/**
 * VC-06 — the selection is a surviving missing record: the field refuses
 * instead of falling back, and the error names the record label.
 *
 * Authorized delta: `WorktreeRefSelect` paints its missing chip only when the
 * id is absent from the array. A tombstone still in the list shows the name
 * on the trigger; `data-state="missing"` and the field error carry the rest.
 */
export const SelectionWentMissing: Story = {
  args: {
    ...Default.args,
    value: MISSING_SELECTION,
    worktrees: [...WORKTREES, worktreeMissingFixture],
  },
  render: () => (
    <Harness initial={MISSING_SELECTION} worktrees={[...WORKTREES, worktreeMissingFixture]} />
  ),
};

/**
 * VC-10 — every production-owned creation status.
 *
 * Authorized delta: the contract row names chip + `WorktreePhaseSteps` (exit
 * `commit · push · pr`), which cannot describe a create. This field is the
 * owner: pending chip, daemon `pending_phase` (`branch` · `checkout` · `copy` ·
 * `setup`), and Cancel. `creating` has no phase — the POST is still in flight
 * or the row is not in the list yet.
 */
export const MaterializingPhase: Story = {
  args: { ...Default.args, value: NEW_TARGET },
  render: () => (
    <StorySurface className="flex w-[420px] flex-col gap-6">
      {CREATION_PHASES.map(live => (
        <FieldRow
          key={`${live.status}-${live.phase ?? ""}`}
          initial={NEW_TARGET}
          materialization={live}
        />
      ))}
    </StorySurface>
  ),
};

/**
 * VC-11 — creation failed and the draft survives it.
 *
 * Authorized delta: production recovery is Retry, Use workspace root, and the
 * still-enabled select (pick another ready worktree). There is no `Pick
 * another…`, `draft kept`, or `Continue at workspace root` control.
 */
export const MaterializationFailedThreeExits: Story = {
  args: { ...Default.args, value: NEW_TARGET },
  render: () => (
    <Harness
      initial={NEW_TARGET}
      materialization={materialization({
        status: "failed",
        error: "bun install exited 1 while bootstrapping docs-refresh.",
      })}
    />
  ),
  tags: ["play-fn"],
  play: OpenWithReadyWorktrees.play,
};
