import type { LaneTabsItem } from "@compozy/ui";

import type { LoopEditorSidebarTab } from "../../hooks/use-loop-editor-state";

// The editor's inspector rail and validation dock, shared with the editor skeleton so the two
// cannot drift. In a module of their own: a component file that also exports constants cannot be
// hot-reloaded without losing component state.

/** The inspector rail's two lanes. */
export const SIDEBAR_TABS: ReadonlyArray<LaneTabsItem<LoopEditorSidebarTab>> = [
  { value: "contract", label: "Contract", testId: "loop-editor-tab-contract" },
  { value: "node", label: "Node", testId: "loop-editor-tab-node" },
];

/** The rail's frame and its tab strip. */
export const SIDEBAR_CLASS = "flex min-h-0 flex-col border-l border-line bg-canvas";
export const SIDEBAR_TABS_CLASS = "flex min-h-0 flex-1 flex-col gap-0";
export const SIDEBAR_TABS_LIST_CLASS = "w-full shrink-0 border-b border-line px-4";

/** The validation dock's frame and its always-visible bar. */
export const DOCK_CLASS = "flex max-h-48 flex-none flex-col border-t border-line bg-sunken";
export const DOCK_BAR_CLASS = "flex h-9 items-center gap-2 px-4";

/** The editor's toolbar row. */
export const EDITOR_TOOLBAR_CLASS =
  "flex min-h-12 flex-none items-center gap-2.5 border-b border-line bg-canvas px-3.5";
