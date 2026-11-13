/**
 * The static states of the draft: each scene names what the bar and the content show and, for a
 * frozen drag, where the pointer is and what it would do.
 */
import type { DragGhostSource } from "../window-tabs-shared/drag-ghost";
import { TAB } from "../window-tabs-shared/fixtures";
import type { DraftTab, DropEdge } from "../window-tabs-shared/model";
import { leaf } from "../window-tabs-shared/model";
import type { SeamState } from "../window-tabs-shared/hooks/use-split-view-options";
import type { OsTabProps } from "../window-tabs-shared/os-tab";
import type { UpgradeNoticeState } from "../window-tabs-shared/upgrade-notice";

interface Ghost {
  source: DragGhostSource;
  /** Pointer position: over the content (percent of it) or over the bar (px in the shell). */
  layer: "content" | "shell";
  left: string;
  top: string;
  refused?: boolean;
}

export interface Scene {
  tabs: readonly DraftTab[];
  activeTabId: string;
  /** The tab whose content shows; differs from the active tab while a drag previews another. */
  contentTab?: DraftTab;
  focusedId?: string;
  tabProps?: Record<string, Partial<OsTabProps>>;
  insertionIndex?: number;
  overflowing?: boolean;
  compact?: boolean;
  seams?: Record<string, SeamState>;
  drop?: { windowId: string; edge: DropEdge; mode?: "split" | "swap" };
  rootDrop?: DropEdge;
  draggingId?: string;
  stacked?: Record<string, string>;
  ghost?: Ghost;
  notice?: UpgradeNoticeState;
}

const BASE = [TAB.dash, TAB.agents, TAB.auth, TAB.twoUp, TAB.review, TAB.docs];
const MANY = [...BASE, TAB.flaky, TAB.term, TAB.files, TAB.fourUp, TAB.threeWay];
const SESSIONS = [TAB.auth, TAB.tabs, TAB.review, TAB.docs, TAB.flaky];
const FLOOR: DraftTab[] = Array.from({ length: 40 }, (_, index) => {
  const source = SESSIONS[index % SESSIONS.length]!;
  return { ...source, id: `${source.id}-${index}` };
});
const EMPTY: DraftTab = { id: "t-empty", root: leaf("dash") };

const dragTab = (tab: DraftTab): DragGhostSource => ({ kind: "tab", tab });
const dragHead = (windowId: string): DragGhostSource => ({ kind: "head", windowId });
const dimmed = (tabId: string) => ({ [tabId]: { dragging: true } });

export const SCENES: Record<string, Scene> = {
  "bar-one": { tabs: [TAB.auth], activeTabId: TAB.auth.id },
  "bar-pinned": {
    tabs: [TAB.dash, TAB.agents, TAB.pinnedSplit, TAB.auth, TAB.twoUp, TAB.review],
    activeTabId: TAB.auth.id,
  },
  "bar-many": { tabs: MANY, activeTabId: TAB.twoUp.id, focusedId: "tabs" },
  "bar-crowded": {
    tabs: [TAB.dash, TAB.agents, ...FLOOR.slice(0, 22)],
    activeTabId: FLOOR[6]!.id,
  },
  "bar-overflow": {
    tabs: [TAB.dash, TAB.agents, ...FLOOR],
    activeTabId: FLOOR[6]!.id,
    overflowing: true,
  },
  "bar-reorder": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    focusedId: "tabs",
    tabProps: dimmed(TAB.review.id),
    insertionIndex: 2,
    ghost: { source: dragTab(TAB.review), layer: "shell", left: "196px", top: "62px" },
  },
  "bar-insertion": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    focusedId: "term",
    draggingId: "term",
    insertionIndex: 5,
    ghost: { source: dragHead("term"), layer: "shell", left: "640px", top: "60px" },
  },
  "bar-dwell": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    contentTab: TAB.auth,
    tabProps: { [TAB.auth.id]: { dwelling: true } },
    drop: { windowId: "auth", edge: "right" },
    ghost: { source: dragHead("term"), layer: "content", left: "72%", top: "38%" },
  },
  "bar-compact": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    compact: true,
    stacked: { "s-two": "tabs" },
  },
  "split-single": { tabs: BASE, activeTabId: TAB.auth.id },
  "split-two": { tabs: BASE, activeTabId: TAB.twoUp.id, focusedId: "tabs" },
  "split-three": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, TAB.threeWay, TAB.review],
    activeTabId: TAB.threeWay.id,
    focusedId: "files",
  },
  "split-four": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, TAB.fourUp, TAB.docs],
    activeTabId: TAB.fourUp.id,
    focusedId: "review",
  },
  "split-stacked": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, TAB.threeWay, TAB.review],
    activeTabId: TAB.threeWay.id,
    focusedId: "files",
    stacked: { "s-three-col": "files" },
  },
  "drop-right": {
    tabs: BASE,
    activeTabId: TAB.auth.id,
    tabProps: dimmed(TAB.review.id),
    drop: { windowId: "auth", edge: "right" },
    ghost: { source: dragTab(TAB.review), layer: "content", left: "70%", top: "40%" },
  },
  "drop-bottom": {
    tabs: BASE,
    activeTabId: TAB.auth.id,
    tabProps: dimmed(TAB.review.id),
    drop: { windowId: "auth", edge: "bottom" },
    ghost: { source: dragTab(TAB.review), layer: "content", left: "46%", top: "78%" },
  },
  "drop-nested": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    focusedId: "tabs",
    tabProps: dimmed(TAB.docs.id),
    drop: { windowId: "term", edge: "bottom" },
    ghost: { source: dragTab(TAB.docs), layer: "content", left: "78%", top: "74%" },
  },
  "drop-root": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    focusedId: "tabs",
    tabProps: dimmed(TAB.docs.id),
    rootDrop: "left",
    ghost: { source: dragTab(TAB.docs), layer: "content", left: "1%", top: "46%" },
  },
  "drop-self": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, TAB.review],
    activeTabId: TAB.review.id,
    contentTab: TAB.auth,
    tabProps: dimmed(TAB.review.id),
    drop: { windowId: "auth", edge: "left" },
    ghost: { source: dragTab(TAB.review), layer: "content", left: "22%", top: "44%" },
  },
  "drop-head": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, TAB.threeWay, TAB.review],
    activeTabId: TAB.threeWay.id,
    focusedId: "term",
    draggingId: "term",
    drop: { windowId: "tabs", edge: "top" },
    ghost: { source: dragHead("term"), layer: "content", left: "24%", top: "16%" },
  },
  "drop-swap": {
    tabs: BASE,
    activeTabId: TAB.twoUp.id,
    focusedId: "term",
    draggingId: "term",
    drop: { windowId: "tabs", edge: "left", mode: "swap" },
    ghost: { source: dragHead("term"), layer: "content", left: "30%", top: "44%" },
  },
  "drop-withheld": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, TAB.fourUp, TAB.docs],
    activeTabId: TAB.fourUp.id,
    focusedId: "review",
    tabProps: dimmed(TAB.docs.id),
    ghost: {
      source: dragTab(TAB.docs),
      layer: "content",
      left: "60%",
      top: "70%",
      refused: true,
    },
  },
  "seam-hover": { tabs: BASE, activeTabId: TAB.twoUp.id, seams: { "s-two:0": "hover" } },
  "seam-drag": { tabs: BASE, activeTabId: TAB.twoUp.id, seams: { "s-two:0": "drag" } },
  "seam-focus": { tabs: BASE, activeTabId: TAB.twoUp.id, seams: { "s-two:0": "focus" } },
  "seam-limit": {
    tabs: [TAB.dash, TAB.agents, TAB.auth, { ...TAB.twoUp, root: narrowTwoUp() }, TAB.review],
    activeTabId: TAB.twoUp.id,
    seams: { "s-two:0": "limit" },
  },
  "notice-migrated": {
    tabs: [TAB.dash, TAB.auth, TAB.twoUp, TAB.review, TAB.docs, TAB.term, TAB.files],
    activeTabId: TAB.twoUp.id,
    notice: "migrated",
  },
  "notice-failed": { tabs: [EMPTY], activeTabId: EMPTY.id, notice: "failed" },
  "notice-retrying": { tabs: [EMPTY], activeTabId: EMPTY.id, notice: "retrying" },
};

function narrowTwoUp() {
  const root = TAB.twoUp.root;
  return root.kind === "split" ? { ...root, weights: [4, 1] } : root;
}
