import { defineFlow } from "#gallery/flow";

/**
 * From the Sessions board to the pull request a session opened: pick the running card, land on
 * its row in List, read the PR chip in the session topbar, ask the agent to fix the failing
 * checks, then open the PR on GitHub.
 */
export const flow = defineFlow({
  title: "Board to pull request",
  spec: "sessions-kanban",
  description:
    "A person scans the board, opens the session working on a draft PR, and follows its pull request chip until the PR is ready on GitHub.",
  start: ["board"],
  nodes: {
    board: { proto: "sessions-board", state: "populated" },
    card: { proto: "session-board-card", state: "running-draft" },
    row: { proto: "session-list-row", state: "current" },
    failing: { proto: "session-pull-request-chip", state: "checks-failing" },
    ready: { proto: "session-pull-request-chip", state: "ready-to-merge" },
    github: {
      external: "github",
      label: "Pull request on GitHub",
      url: "https://github.com/compozy/compozy/pull/932",
      end: true,
    },
  },
  edges: [
    ["board", "card", { on: "Topbar PR chip — web slice", kind: "overlay" }],
    ["card", "row", { on: "Topbar PR chip — web slice" }],
    ["row", "failing", { on: "Topbar PR chip — web slice" }],
    ["failing", "ready", { on: "Fix failing checks", kind: "overlay" }],
    ["failing", "github", { on: "Open PR", kind: "external" }],
    ["ready", "github", { on: "Open PR", kind: "external" }],
  ],
});
