import { leaf, split, type DraftDesktop, type DraftTab, type DraftWindow } from "./model";

export const WINDOWS: Record<string, DraftWindow> = Object.fromEntries(
  (
    [
      { id: "auth", app: "session", title: "Fix login redirect loop", provider: "claude" },
      {
        id: "tabs",
        app: "session",
        title: "Window tabs and splits",
        provider: "claude",
        signal: "running",
      },
      {
        id: "review",
        app: "session",
        title: "Review PR #1609",
        provider: "codex",
        signal: "attention",
        pending: 2,
      },
      { id: "docs", app: "session", title: "Release notes v0.7.0", provider: "gemini" },
      { id: "flaky", app: "session", title: "Flaky attach test", provider: "codex" },
      { id: "term", app: "terminal", title: "make dev" },
      { id: "term-tests", app: "terminal", title: "bun test --watch" },
      { id: "files", app: "files", title: "web/src/systems/os" },
      { id: "dash", app: "dashboard", title: "Dashboard" },
      { id: "agents", app: "agents", title: "Agents" },
      { id: "auto", app: "automations", title: "Automations" },
    ] satisfies DraftWindow[]
  ).map(window => [window.id, window])
);

export const TAB = {
  dash: { id: "t-dash", pinned: true, root: leaf("dash") },
  agents: { id: "t-agents", pinned: true, root: leaf("agents") },
  pinnedSplit: {
    id: "t-pinned-split",
    pinned: true,
    root: split("s-pin", "row", [leaf("auto"), leaf("term-tests")]),
  },
  auth: { id: "t-auth", root: leaf("auth") },
  tabs: { id: "t-tabs", root: leaf("tabs") },
  review: { id: "t-review", root: leaf("review") },
  docs: { id: "t-docs", root: leaf("docs") },
  flaky: { id: "t-flaky", root: leaf("flaky") },
  term: { id: "t-term", root: leaf("term") },
  files: { id: "t-files", root: leaf("files") },
  twoUp: { id: "t-two", root: split("s-two", "row", [leaf("tabs"), leaf("term")], [3, 2]) },
  threeWay: {
    id: "t-three",
    root: split("s-three", "row", [
      leaf("tabs"),
      split("s-three-col", "column", [leaf("files"), leaf("term")], [3, 2]),
    ]),
  },
  fourUp: {
    id: "t-four",
    root: split("s-four", "row", [
      split("s-four-a", "column", [leaf("review"), leaf("flaky")]),
      split("s-four-b", "column", [leaf("files"), leaf("term-tests")]),
    ]),
  },
} satisfies Record<string, DraftTab>;

export const DESKTOPS: readonly DraftDesktop[] = [
  {
    id: "control",
    name: "Control",
    tabs: [TAB.dash, TAB.agents],
    activeTabId: TAB.dash.id,
  },
  {
    id: "build",
    name: "Build",
    tabs: [TAB.dash, TAB.auth, TAB.twoUp, TAB.review, TAB.docs],
    activeTabId: TAB.twoUp.id,
  },
  {
    id: "review",
    name: "Review",
    tabs: [TAB.fourUp, TAB.flaky],
    activeTabId: TAB.fourUp.id,
    needsYou: true,
  },
];
