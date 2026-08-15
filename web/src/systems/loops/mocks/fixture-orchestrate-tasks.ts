import { fixtureGraph } from "./fixture-graph";

export const orchestrateTasksGraph = fixtureGraph(
  [
    { id: "slug_input", class: "source", kind: "input", input_ref: "slug" },
    {
      id: "orchestrate",
      class: "action",
      kind: "goal",
      session: { mode: "continuous" },
      params: {
        agent: "{{ .inputs.orchestrator }}",
        objective:
          "Conduct the authored task graph in dependency order, using one bounded worker session per task.",
        output_schema: {
          type: "object",
          required: ["status", "summary"],
          properties: {
            status: { enum: ["completed", "blocked"] },
            summary: { type: "string" },
            tasks: { type: "array" },
          },
        },
      },
    },
  ],
  [{ from: "slug_input", to: "orchestrate" }]
);
