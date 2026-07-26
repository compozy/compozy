import type { Meta, StoryObj } from "@storybook/react-vite";
import { HttpResponse } from "msw";
import type { ComponentProps } from "react";

import { aghApiMock } from "@/storybook/openapi-msw";
import { StorySurface, StoryTopbarHost } from "@/storybook/story-layout";

import { LoopEditor } from "../editor/loop-editor";
import { handlers as loopHandlers } from "../../mocks";
import { loopDetailByName } from "../../mocks/fixtures";
import type { LoopDetail } from "../../types";

const WS = "ws_default";
const delivery = loopDetailByName.get("software-delivery")!;

type RawGraph = { nodes: Record<string, unknown>[]; edges: unknown[] };

/** A software-delivery detail whose fan-out node breaches the ceiling, so the editor's
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

/** Long tool kind + id so canvas cards prove ellipsis stays inside the fixed 132px width. */
function longKindDetail(): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const nodes = graph.nodes.map(node =>
    node.id === "execute_task"
      ? {
          ...node,
          id: "resolve_threads",
          kind: "ext__dev_cycle__coderabbit_resolve_threads",
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
    aghApiMock.get("/api/workspaces/{workspace_id}/loops/{name}", () =>
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

/** The clean editor over a workspace software-delivery Loop: canvas, palette,
 *  inspector, linter dock (all invariants pass), and the read-only Start summary. */
export const Editor: Story = {
  args: { workspaceId: WS, name: "software-delivery" },
};

/** Goal authoring block selected in the inspector, including the closed judge,
 *  exhaustion, continuous-session, and pre-submit retry fields. */
export const GoalBlock: Story = {
  args: { workspaceId: WS, name: "software-delivery" },
  parameters: { msw: { handlers: editorHandlers(goalDetail()) } },
  render: args => <EditorHarness {...args} heightClass="h-[1100px]" />,
};

/** A fan-out node over the daemon ceiling: the shared linter returns a per-node 422 —
 *  the fan-out chip fails, the node gets a danger ring + badge, and Publish is disabled. */
export const FanOutError: Story = {
  args: { workspaceId: WS, name: "software-delivery" },
  parameters: { msw: { handlers: editorHandlers(overCeilingDetail()) } },
};

/** Editing a read-only watch Loop before a workspace fork exists. */
export const WatchFork: Story = {
  args: { workspaceId: WS, name: "reviews-watch" },
};

/** Canvas node id/kind ellipsis inside the fixed-width card (long extension tool ids). */
export const LongKindLabels: Story = {
  args: { workspaceId: WS, name: "software-delivery" },
  parameters: { msw: { handlers: editorHandlers(longKindDetail()) } },
};
