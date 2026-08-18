import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import type { ComponentProps } from "react";
import { expect, userEvent, waitFor, within } from "storybook/test";

import { compozyApiMock } from "@/storybook/openapi-msw";
import { StorySurface, StoryTopbarHost } from "@/storybook/story-layout";

import { LoopEditor } from "../editor/loop-editor";
import { LoopEditorConnectionPicker } from "../editor/loop-editor-connection-picker";
import { handlers as loopHandlers, RELEASE_TRAIN_LOOP_NAME, releaseTrainDetail } from "../../mocks";
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

function unboundedFanOutDetail(): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const nodes = graph.nodes.map(node =>
    node.id === "implement" ? { ...node, max_fan_out: 0 } : node
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

export const Editor: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
};

export const DslView: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-editor-view-dsl"));
  },
};

export const GoalBlock: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(goalDetail()) } },
  render: args => <EditorHarness {...args} heightClass="h-[1100px]" />,
};

export const FanOutError: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(unboundedFanOutDetail()) } },
};

export const PackagedFork: Story = {
  args: { workspaceId: WS, name: "review-and-fix" },
};

export const LongKindLabels: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(longKindDetail()) } },
};

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

export const DirtyCustom: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-palette-item-collect"));
  },
};

export const NodeReliability: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: selectNode("execute_task", ["Reliability", "Reactions"], 72),
};

export const ContractTerminals: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-editor-contract-terminals");
    await revealFolds(canvasElement, ["Terminal reactions"]);
  },
};

export const WaitInspector: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: selectNode("await_deploy_ack", ["Expiry"]),
};

export const RunLoopParentClose: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(fullLifecycleDetail) } },
  play: selectNode("release_notes"),
};

export const LintErrorAndWarning: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(lintErrorAndWarningDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-linter-error-count");
    await selectNode("implement")({ canvasElement });
    await userEvent.click(canvas.getByTestId("loop-linter-toggle"));
  },
};

export const LintWarningOnly: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(waitWarningDetail) } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-linter-warning-count");
    await userEvent.click(canvas.getByTestId("loop-linter-toggle"));
  },
};

export const LintClean: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await userEvent.click(await canvas.findByTestId("loop-linter-toggle"));
  },
};

export const ReadOnlySource: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(readOnlySourceDetail) } },
};

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

export const NodeEnvironmentRoot: Story = {
  args: { workspaceId: WS, name: "implement-tasks" },
  parameters: { msw: { handlers: editorHandlers(nodeEnvironmentDetail({ mode: "root" })) } },
  play: async ({ canvasElement }) => {
    const canvas = await revealNodeEnvironment(canvasElement);
    await canvas.findByText("Runs at the workspace root. Part of the session binding key.");
  },
};

export const NodeEnvironmentWorktree: Story = {
  args: { workspaceId: storyWorkspaceIds.hq, name: "implement-tasks" },
  parameters: {
    msw: {
      handlers: [
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

function releaseTrainHandlers() {
  return editorHandlers(releaseTrainDetail);
}

const releaseTrainArgs = { workspaceId: WS, name: RELEASE_TRAIN_LOOP_NAME };

export const ChromeCalmDefault: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
};

export const ChromePaletteExpanded: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    await canvas.findByTestId("loop-editor");
    await userEvent.click(canvas.getByTestId("loop-editor-palette-toggle"));
    await userEvent.type(await canvas.findByTestId("loop-palette-search"), "ro");
  },
};

export const RouteInspector: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: selectNode("triage"),
};

export const AskInspector: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: selectNode("confirm-rollout"),
};

export const StrategyInspector: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: selectNode("rollout"),
};

export const ReviewInspectorWithDock: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: async ({ canvasElement }) => {
    await selectNode("apply-migration")({ canvasElement });
    const canvas = within(canvasElement);
    await userEvent.click(canvas.getByTestId("loop-linter-toggle"));
  },
};

export const QuickAdd: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const card = (await canvas.findAllByTestId("loop-editor-node"))[0];
    await userEvent.click(card);
    await userEvent.keyboard("a");
    await waitFor(() => expect(document.querySelector("[data-testid='loop-editor-quick-add']")));
  },
};

export const NodeContextMenu: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  play: async ({ canvasElement }) => {
    const canvas = within(canvasElement);
    const cards = await canvas.findAllByTestId("loop-editor-node");
    const card = cards.find(element => element.getAttribute("data-node-id") === "triage");
    if (!card) throw new Error("node card triage not found");
    await userEvent.pointer({ keys: "[MouseRight]", target: card });
  },
};

export const ConnectionDropPicker: Story = {
  args: releaseTrainArgs,
  parameters: { msw: { handlers: releaseTrainHandlers() } },
  render: () => (
    <StoryTopbarHost title="Editor">
      <StorySurface className="flex h-[560px] items-center justify-center p-0">
        <LoopEditorConnectionPicker
          onOpenChange={() => {}}
          onPick={() => {}}
          open
          point={{ x: 420, y: 260 }}
          sourceNodeId="triage"
        />
      </StorySurface>
    </StoryTopbarHost>
  ),
};
