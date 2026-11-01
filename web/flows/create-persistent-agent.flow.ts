import { defineFlow } from "@compozy/proto/flow";

/**
 * From New agent to the avatar above a project's sessions: pick Persistent, walk the wizard, place
 * the agent in compozy, see it in the sidebar, then open its first report. S16's two pickers branch
 * from the sidebar's project `+` and from a card's `+`.
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
    "add-menu": { proto: "persistent-sessions", state: "sidebar-add-menu" },
    picker: { proto: "persistent-sessions", state: "add-picker" },
    "picker-none": { proto: "persistent-sessions", state: "add-picker-none", end: true },
    "picker-cap": { proto: "persistent-sessions", state: "add-picker-cap", end: true },
    "picker-auth": { proto: "persistent-sessions", state: "add-picker-auth", end: true },
    preselected: { proto: "agent-create-wizard", state: "preselected-scope", end: true },
    scope: { proto: "persistent-agents-list", state: "add-scope" },
    "scope-none": { proto: "persistent-agents-list", state: "add-scope-none", end: true },
    "scope-cap": { proto: "persistent-agents-list", state: "add-scope-cap", end: true },
    "scope-auth": { proto: "persistent-agents-list", state: "add-scope-auth", end: true },
    "profile-scope": { proto: "agent-profile", state: "where-add-scope", end: true },
  },
  edges: [
    ["list", "kind", { on: "New agent", kind: "overlay" }],
    ["kind", "identity", { on: "Persistent" }],
    ["identity", "runtime", { on: "Continue" }],
    ["runtime", "persona", { on: "Continue" }],
    ["persona", "where", { on: "Continue" }],
    ["where", "sidebar", { on: "Create and start", kind: "redirect" }],
    ["sidebar", "report", { on: "Triage Lead" }],
    ["sidebar", "add-menu", { on: "Add to compozy", kind: "overlay" }],
    ["add-menu", "picker", { on: "Add persistent agent", kind: "overlay" }],
    ["add-menu", "picker-none", { on: "Add persistent agent", kind: "overlay" }],
    ["add-menu", "picker-cap", { on: "Add persistent agent", kind: "overlay" }],
    ["add-menu", "picker-auth", { on: "Add persistent agent", kind: "overlay" }],
    ["picker", "preselected", { on: "Create new" }],
    ["list", "scope", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-none", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-cap", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-auth", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["scope", "profile-scope", { on: "Triage Lead" }],
  ],
});
