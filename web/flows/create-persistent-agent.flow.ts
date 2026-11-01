import { defineFlow } from "@compozy/proto/flow";

/**
 * From New agent to the avatar above a project's sessions: pick Persistent, walk the wizard, place
 * the agent in compozy, see it in the sidebar, then open its first report.
 */
export const flow = defineFlow({
  title: "Create a persistent agent",
  spec: "always-on-agent",
  description:
    "A person creates a persistent agent, places it in a workspace, finds its avatar above the project's sessions, and reads the proposal it sent.",
  start: ["list"],
  nodes: {
    list: { proto: "persistent-agents-list", state: "cards" },
    kind: { proto: "agent-create-wizard", state: "kind" },
    identity: { proto: "agent-create-wizard", state: "persistent-identity" },
    runtime: { proto: "agent-create-wizard", state: "persistent-runtime" },
    persona: { proto: "agent-create-wizard", state: "persistent-persona" },
    where: { proto: "agent-create-wizard", state: "persistent-where" },
    sidebar: { proto: "persistent-sessions", state: "sidebar-rows" },
    report: { proto: "persistent-reports", state: "proposal", end: true },
  },
  edges: [
    ["list", "kind", { on: "New agent", kind: "overlay" }],
    ["kind", "identity", { on: "Persistent" }],
    ["identity", "runtime", { on: "Continue" }],
    ["runtime", "persona", { on: "Continue" }],
    ["persona", "where", { on: "Continue" }],
    ["where", "sidebar", { on: "Create and start", kind: "redirect" }],
    ["sidebar", "report", { on: "Triage Lead" }],
  ],
});
