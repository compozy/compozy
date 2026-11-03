import { defineFlow } from "@compozy/proto/flow";

/**
 * From New agent to the avatar above a project's sessions: fill the one create screen, choose that
 * the agent is always-on, pick the project, see it in the sidebar, then open its first report. S16's two
 * pickers branch from the sidebar's project `+` and from a card's `+`.
 */
export const flow = defineFlow({
  title: "Create an always-on agent",
  spec: "always-on-agent",
  description:
    "A person creates an always-on agent, picks where it works, finds its avatar above the project's sessions, and reads the proposal it sent.",
  start: ["list"],
  nodes: {
    list: { proto: "always-on-agents-list", state: "cards" },
    empty: { proto: "agent-create", state: "empty" },
    "on-demand": { proto: "agent-create", state: "filled", end: true },
    "always-on": { proto: "agent-create", state: "always-on" },
    where: { proto: "agent-create", state: "where-search" },
    sidebar: { proto: "always-on-sessions", state: "sidebar-rows" },
    report: { proto: "always-on-reports", state: "proposal", end: true },
    "add-menu": { proto: "always-on-sessions", state: "sidebar-add-menu" },
    picker: { proto: "always-on-sessions", state: "add-picker" },
    "picker-none": { proto: "always-on-sessions", state: "add-picker-none", end: true },
    "picker-cap": { proto: "always-on-sessions", state: "add-picker-cap", end: true },
    "picker-auth": { proto: "always-on-sessions", state: "add-picker-auth", end: true },
    "from-project": { proto: "agent-create", state: "from-project", end: true },
    scope: { proto: "always-on-agents-list", state: "add-scope" },
    "scope-none": { proto: "always-on-agents-list", state: "add-scope-none", end: true },
    "scope-cap": { proto: "always-on-agents-list", state: "add-scope-cap", end: true },
    "scope-auth": { proto: "always-on-agents-list", state: "add-scope-auth", end: true },
    "profile-scope": { proto: "agent-profile", state: "where-add-scope", end: true },
  },
  edges: [
    ["list", "empty", { on: "New agent", kind: "overlay" }],
    ["empty", "on-demand", { on: "On-demand: runs when you ask" }],
    ["empty", "always-on", { on: "Always-on: watches for you" }],
    ["always-on", "where", { on: "Where it works", kind: "overlay" }],
    ["where", "sidebar", { on: "Create and start", kind: "redirect" }],
    ["sidebar", "report", { on: "Triage Lead" }],
    ["sidebar", "add-menu", { on: "Add to compozy", kind: "overlay" }],
    ["add-menu", "picker", { on: "Add always-on agent", kind: "overlay" }],
    ["add-menu", "picker-none", { on: "Add always-on agent", kind: "overlay" }],
    ["add-menu", "picker-cap", { on: "Add always-on agent", kind: "overlay" }],
    ["add-menu", "picker-auth", { on: "Add always-on agent", kind: "overlay" }],
    ["picker", "from-project", { on: "Create new" }],
    ["list", "scope", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-none", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-cap", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["list", "scope-auth", { on: "Add Triage Lead to a workspace", kind: "overlay" }],
    ["scope", "profile-scope", { on: "Triage Lead" }],
  ],
});
