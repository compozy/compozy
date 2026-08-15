import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import type { ComponentProps } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { StorySurface, StoryTopbarHost } from "@/storybook/story-layout";

import { LoopEditor } from "../editor/loop-editor";
import { handlers as loopHandlers } from "../../mocks";
import {
  PUBLISH_REJECTED_ISSUES,
  fullLifecycleDetail,
  lintErrorAndWarningDetail,
  readOnlySourceDetail,
  waitWarningDetail,
} from "../../mocks/fixture-editor-lifecycle";
import {
  loopConfigFixture,
  loopDetailByName,
  loopEffectiveConfigFixture,
} from "../../mocks/fixtures";
import type { LoopDetail } from "../../types";
import {
  nodeEnvironmentDetail,
  RETIRED_CWD_ISSUES,
  retiredCwdDetail,
  revealNodeEnvironment,
} from "./loop-editor-environment-story-helpers";

import { storyWorkspaceIds } from "@/storybook/fintech-scenario";
import {
  worktreeBehindFixture,
  worktreeHandlers,
  worktreeListingFixture,
} from "@/systems/workspace/mocks";

const WS = "ws_default";
const delivery = loopDetailByName.get("implement-tasks")!;

type RawGraph = { nodes: Record<string, unknown>[]; edges: unknown[] };

/** A implement-tasks detail whose fan-out node breaches the ceiling, so the editor's
 *  auto-validate surfaces the fan_out_ceiling_exceeded issue + node badge + gate. */
function overCeilingDetail(): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const nodes = graph.nodes.map(node =>
    node.id === "implement" ? { ...node, max_fan_out: 80 } : node
  );
  return {
    ...delivery,
    definition: {
      ...delivery.definition,
      graph: { ...graph, nodes } as unknown as LoopDetail["definition"]["graph"],
    },
  };
}

function goalDetail(): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const goal = {
    id: "ship_goal_surfaces",
    class: "action",
    kind: "goal",
    params: {
      agent: "implementer",
      objective: "Ship the complete Goal operator surfaces.",
      judge: [{ id: "verified", type: "command", check: "make verify", expect: "exit_zero" }],
      max_turns: 20,
      on_exhausted: "halt",
    },
    session: { mode: "continuous" },
    retry: { max_attempts: 2, on_failure: "fresh_session" },
  };
  return {
    ...delivery,
    definition: {
      ...delivery.definition,
      graph: {
        ...graph,
        nodes: [goal, ...graph.nodes],
      } as unknown as LoopDetail["definition"]["graph"],
    },
  };
}

/** Long tool kind + id so canvas cards prove ellipsis stays inside the fixed 188px width. */
function longKindDetail(): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const nodes = graph.nodes.map(node =>
    node.id === "execute_task"
      ? {
          ...node,
          id: "collect_review_artifact_evidence",
          kind: "ext__example_extension__collect_review_artifact_evidence",
        }
      : node
  );
  return {
    ...delivery,
    definition: {
      ...delivery.definition,
      graph: { ...graph, nodes } as unknown as LoopDetail["definition"]["graph"],
    },
  };
}

function editorHandlers(detail: LoopDetail) {
  // The override getLoop must win, but keep the loop handlers (validate lints the posted
  // definition) so the auto-validate still surfaces the fan_out_ceiling_exceeded issue.
  return [
    compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}", () =>
      HttpResponse.json({ loop: detail })
    ),
    ...loopHandlers,
  ];
}

function EditorHarness({
  heightClass = "h-[880px]",
  ...args
}: ComponentProps<typeof LoopEditor> & { heightClass?: string }) {
  return (
    <StoryTopbarHost title="Editor">
      <StorySurface className={`flex ${heightClass} p-0`}>
        <LoopEditor {...args} />
      </StorySurface>
    </StoryTopbarHost>
  );
}

const meta: Meta<typeof LoopEditor> = {
  title: "systems/loops/components/LoopEditor",
  component: LoopEditor,
  parameters: { layout: "fullscreen" },
  render: args => <EditorHarness {...args} />,
};

export default meta;
type Story = StoryObj<typeof meta>;

/** The clean editor over a workspace implement-tasks Loop: canvas, palette,
 *  inspector, linter dock (all invariants pass), and the read-only Start summary. */
export const Editor: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
};

/** The same canonical Loop definition rendered through the read-only DSL view. */
export const DslView: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-editor-view-dsl"));
  },
};

/** Goal authoring block selected in the inspector, including the closed judge,
 *  exhaustion, continuous-session, and pre-submit retry fields. */
export const GoalBlock: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(goalDetail()) } },
  render: args => <EditorHarness {...args} heightClass="h-[1100px]" />,
};

/** A fan-out node over the daemon ceiling: the shared linter returns a per-node 422 —
 *  the fan-out chip fails, the node gets a danger ring + badge, and Publish is disabled. */
export const FanOutError: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(overCeilingDetail()) } },
};

/** Editing the packaged review Loop before a workspace fork exists. */
export const PackagedFork: Story = {
  args: { workspaceId: WS, name: "review-and-fix" },
};

/** Canvas node id/kind ellipsis inside the fixed-width card (long extension tool ids). */
export const LongKindLabels: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(longKindDetail()) } },
};

/** Opens the named inspector folds and scrolls the final one into view. */
async function revealFolds(canvasElement: HTMLElement, labels: string[], scrollOffset = 0) {
  const triggers = Array.from(canvasElement.querySelectorAll("button, summary"));
  let last: HTMLElement | undefined;
  for (const label of labels) {
    const trigger = triggers.find(element => element.textContent?.startsWith(label));
    if (!trigger) throw new Error(`fold ${label} not found`);
    const expanded =
      trigger.getAttribute("aria-expanded") === "true" ||
      trigger.closest("[data-open]") !== null ||
      (trigger.parentElement as HTMLDetailsElement | null)?.open === true;
    if (!expanded) {
      await userEvent.click(trigger);
    }
    last = trigger as HTMLElement;
  }
  last?.scrollIntoView({ block: "start" });
  const scrollParent = last?.closest(".overflow-y-auto");
  if (scrollParent instanceof HTMLElement && scrollOffset > 0) {
    scrollParent.scrollTop = Math.max(0, scrollParent.scrollTop - scrollOffset);
  }
}

/** Selects one canvas node so its inspector fields render, then reveals the named folds. */
function selectNode(id: string, folds: string[] = [], scrollOffset = 0) {
  return async ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const canvas = within(canvasElement);
    const cards = await canvas.findAllByTestId("loop-editor-node");
    const card = cards.find(element => element.getAttribute("data-node-id") === id);
    if (!card) throw new Error(`node card ${id} not found`);
    await userEvent.click(card);
    if (folds.length > 0) {
      await canvas.findByTestId("loop-editor-inspector");
      await revealFolds(canvasElement, folds, scrollOffset);
    }
  };
}

/** VC-E1 — a custom Loop with unpublished edits: the dirty version pill at rest. */
export const DirtyCustom: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-palette-item-collect"));
  },
};

/** VC-E2 — the Node inspector: reliability envelope, on_error, and the six triggers. */
export const NodeReliability: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: selectNode("execute_task", ["Reliability", "Reactions"], 72),
};

/** VC-E3 — the Contract lane's seven terminal reactions; unauthored lists stay calm. */
export const ContractTerminals: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-editor-contract-terminals");
    await revealFolds(canvasElement, ["Terminal reactions"]);
  },
};

/** VC-E4 — the wait inspector: mode XOR, expect, ahead_arrival, and the expiry fold. */
export const WaitInspector: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: selectNode("await_deploy_ack", ["Expiry"]),
};

/** VC-E5 — the run-loop child close policy. */
export const RunLoopParentClose: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: selectNode("release_notes"),
};

/** VC-E6 — the dock carrying one blocking error and one warning, expanded. */
export const LintErrorAndWarning: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(lintErrorAndWarningDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-linter-error-count");
    // Select the offending node so the rail matches the reference's Node lane.
    await selectNode("implement")({ canvasElement });
    await userEvent.click(canvas.getByTestId("loop-linter-toggle"));
  },
};

/** A warning-only verdict: visible, with Publish still enabled. */
export const LintWarningOnly: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(waitWarningDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-linter-warning-count");
    await userEvent.click(canvas.getByTestId("loop-linter-toggle"));
  },
};

/** A clean verdict: the Validation eyebrow carries no counter at all. */
export const LintClean: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-linter-toggle"));
  },
};

/** VC-E7 — a read-only marketplace source: every edit path closed, Fork the one verb. */
export const ReadOnlySource: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(readOnlySourceDetail) } },
};

/** VC-E8 — publish rejected: the danger strip lists the daemon's issues; the version holds. */
export const PublishRejected: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: [
        compozyApiMock.patch("/api/workspaces/{workspace_id}/loops/{name}", () =>
          HttpResponse.json({ valid: false, errors: PUBLISH_REJECTED_ISSUES }, { status: 422 })
        ),
        ...editorHandlers(delivery),
      ],
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-palette-item-collect"));
    const publish = await canvas.findByTestId("loop-editor-publish");
    await userEvent.click(publish);
    await canvas.findByTestId("loop-editor-publish-error");
  },
};

/**
 * VC-31 — the one environment control on an agent-executing node, at the
 * workspace root. Non-agent nodes get no control at all, which is why the
 * inspector is opened on `execute_task` rather than the fan-out above it.
 */
export const NodeEnvironmentRoot: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(nodeEnvironmentDetail({ mode: "root" })) } },
  play: async ({ canvasElement }) => {
    const canvas = await revealNodeEnvironment(canvasElement);
    await canvas.findByText("Runs at the workspace root. Part of the session binding key.");
  },
};

/** VC-32 — Named worktree: the mode brings its ready worktree picker with it. */
export const NodeEnvironmentWorktree: Story = {
  args: { workspaceId: storyWorkspaceIds.hq, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: [
        // Story-level `handlers` arrays replace preview groups, so the git-backed
        // listing must be served here — otherwise the picker resolves as missing.
        compozyApiMock.get("/api/workspaces/{workspace_id}/worktrees", () =>
          HttpResponse.json(worktreeListingFixture)
        ),
        ...worktreeHandlers,
        compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}/config", () =>
          HttpResponse.json({
            config: { ...loopConfigFixture, environment: { mode: "root" } },
            effective_config: { ...loopEffectiveConfigFixture, environment: { mode: "root" } },
          })
        ),
        ...editorHandlers(
          nodeEnvironmentDetail({ mode: "worktree", worktree_ref: worktreeBehindFixture.id })
        ),
      ],
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = await revealNodeEnvironment(canvasElement, ["loop-field-environment-ref"]);
    await canvas.findByText("Overrides the loop default (Workspace root) for this node.");
    const picker = canvas.getByTestId("loop-field-environment-ref");
    await waitFor(() => {
      expect(picker).not.toHaveAttribute("data-missing");
    });
    expect(picker).toHaveTextContent(worktreeBehindFixture.name);
    expect(picker).not.toHaveTextContent(worktreeBehindFixture.id);
    expect(within(picker).queryByText("missing")).not.toBeInTheDocument();
  },
};

/** VC-33 — Directory, the escape hatch, holding a value templated over the namespace. */
export const NodeEnvironmentDirectory: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: editorHandlers(
        nodeEnvironmentDetail({ mode: "directory", directory: "packages/{{ .inputs.slug }}" })
      ),
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = await revealNodeEnvironment(canvasElement, ["loop-field-environment-directory"]);
    await canvas.findByRole("combobox", { name: "Directory" });
    await expect(canvas.getByTestId("loop-field-environment-directory")).toHaveValue(
      "packages/{{ .inputs.slug }}"
    );
  },
};

/**
 * VC-34 — the effective environment read back on the node card.
 *
 * The node declares nothing, so the card names the loop default it inherits and
 * marks the readout as coming from there rather than from the node.
 */
export const NodeEnvironmentReadout: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: [
        compozyApiMock.get("/api/workspaces/{workspace_id}/loops/{name}/config", () =>
          HttpResponse.json({
            config: { ...loopConfigFixture, environment: { mode: "per_run" } },
            effective_config: { ...loopEffectiveConfigFixture, environment: { mode: "per_run" } },
          })
        ),
        ...editorHandlers(nodeEnvironmentDetail(null)),
      ],
    },
  },
};

/** VC-35 — a definition still carrying the retired `params.cwd`: the daemon refuses it. */
export const RetiredCwdRejected: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: [
        compozyApiMock.post("/api/workspaces/{workspace_id}/loops/{name}/validate", () =>
          HttpResponse.json({ valid: false, errors: RETIRED_CWD_ISSUES }, { status: 422 })
        ),
        ...editorHandlers(retiredCwdDetail()),
      ],
    },
  },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-linter-error-count");
    await selectNode("execute_task")({ canvasElement });
    await userEvent.click(canvas.getByTestId("loop-linter-toggle"));
  },
};
