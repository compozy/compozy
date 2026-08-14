import { userEvent, within } from "storybook/test";

import { loopDetailByName } from "../../mocks/fixtures";
import type { LoopDetail } from "../../types";

type RawGraph = { nodes: Record<string, unknown>[]; edges: unknown[] };

const delivery = loopDetailByName.get("implement-tasks")!;
const ENVIRONMENT_FIELD_SLOT = '[data-slot="loop-node-environment-field"]';

/**
 * Adds an environment descriptor without replacing the run-agent node's other
 * authored parameters.
 */
export function nodeEnvironmentDetail(environment: Record<string, unknown> | null): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const nodes = graph.nodes.map(node => {
    if (node.id !== "execute_task") return node;
    const params = { ...(node.params as Record<string, unknown> | undefined) };
    if (environment) params.environment = environment;
    else delete params.environment;
    return { ...node, params };
  });
  return {
    ...delivery,
    definition: {
      ...delivery.definition,
      graph: { ...graph, nodes } as unknown as LoopDetail["definition"]["graph"],
    },
  };
}

/** A definition that still carries the retired `params.cwd` (B-009). */
export function retiredCwdDetail(): LoopDetail {
  const graph = delivery.definition.graph as unknown as RawGraph;
  const nodes = graph.nodes.map(node =>
    node.id === "execute_task"
      ? {
          ...node,
          params: { ...(node.params as Record<string, unknown> | undefined), cwd: "packages/api" },
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

export const RETIRED_CWD_ISSUES = [
  {
    node_id: "execute_task",
    code: "environment_cwd_removed",
    message: "params.cwd is retired; use params.environment.directory",
    severity: "error" as const,
  },
];

/** Opens the node and positions its environment control for visual capture. */
export async function revealNodeEnvironment(
  canvasElement: HTMLElement,
  extraTestIds: readonly string[] = []
) {
  const canvas = within(canvasElement);
  const cards = await canvas.findAllByTestId("loop-editor-node");
  const card = cards.find(element => element.getAttribute("data-node-id") === "execute_task");
  if (!card) throw new Error("node card execute_task not found");
  await userEvent.click(card);
  await canvas.findByTestId("loop-field-environment");
  for (const testId of extraTestIds) {
    await canvas.findByTestId(testId);
  }
  const control = canvasElement.querySelector(ENVIRONMENT_FIELD_SLOT);
  if (!(control instanceof HTMLElement)) {
    throw new Error("environment field slot not found");
  }
  control.scrollIntoView({ block: "center" });
  for (const testId of extraTestIds) {
    canvas.getByTestId(testId).scrollIntoView({ block: "nearest" });
  }
  return canvas;
}
