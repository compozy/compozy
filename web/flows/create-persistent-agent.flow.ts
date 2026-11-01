import { defineFlow } from "@compozy/proto/flow";

/**
 * From New agent to the avatar above a project's sessions: fill the one create screen, choose that
 * the agent stays on, pick the project, see it in the sidebar, then open its first report. S16's two
 * pickers branch from the sidebar's project `+` and from a card's `+`.
 */
export const flow = defineFlow({
  title: "Create a persistent agent",
  spec: "always-on-agent",
  description:
    "A person creates an agent that stays on, picks where it works, finds its avatar above the project's sessions, and reads the proposal it sent.",
  start: ["list"],
  nodes: {
    list: { proto: "persistent-agents-list", state: "cards" },
    empty: { proto: "agent-create", state: "empty" },
    "on-demand": { proto: "agent-create", state: "filled", end: true },
    "stays-on": { proto: "agent-create", state: "stays-on" },
    where: { proto: "agent-create", state: "where-search" },
    sidebar: { proto: "persistent-sessions", state: "sidebar-rows" },
    report: { proto: "persistent-reports", state: "proposal", end: true },
    "add-menu": { proto: "persistent-sessions", state: "sidebar-add-menu" },
    picker: { proto: "persistent-sessions", state: "add-picker" },
    "picker-none": { proto: "persistent-sessions", state: "add-picker-none", end: true },
    "picker-cap": { proto: "persistent-sessions", state: "add-picker-cap", end: true },
    "picker-auth": { proto: "persistent-sessions", state: "add-picker-auth", end: true },
    "from-project": { proto: "agent-create", state: "from-project", end: true },
    scope: { proto: "persistent-agents-list", state: "add-scope" },
    "scope-none": { proto: "persistent-agents-list", state: "add-scope-none", end: true },
    "scope-cap": { proto: "persistent-agents-list", state: "add-scope-cap", end: true },
    "scope-auth": { proto: "persistent-agents-list", state: "add-scope-auth", end: true },
    "profile-scope": { proto: "agent-profile", state: "where-add-scope", end: true },
  },
  edges: [
    ["list", "empty", { on: "New agent", kind: "overlay" }],
    ["empty", "on-demand", { on: "Runs when you ask" }],
    ["empty", "stays-on", { on: "Stays on and watches for you" }],
    ["stays-on", "where", { on: "Where it works", kind: "overlay" }],
    ["where", "sidebar", { on: "Create and start", kind: "redirect" }],
    ["sidebar", "report", { on: "Triage Lead" }],
    ["sidebar", "add-menu", { on: "Add to compozy", kind: "overlay" }],
    ["add-menu", "picker", { on: "Add persistent agent", kind: "overlay" }],
    ["add-menu", "picker-none", { on: "Add persistent agent", kind: "overlay" }],
    ["add-menu", "picker-cap", { on: "Add persistent agent", kind: "overlay" }],
    ["add-menu", "picker-auth", { on: "Add persistent agent", kind: "overlay" }],
    ["picker", "from-project", { on: "Create new" }],
    ["list", "scope", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-none", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-cap", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-auth", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["scope", "profile-scope", { on: "Triage Lead" }],
  ],
});
