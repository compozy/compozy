import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { DragGhost } from "../window-tabs-shared/drag-ghost";
import { DESKTOPS, TAB, WINDOWS } from "../window-tabs-shared/fixtures";
import type { DraftDesktop } from "../window-tabs-shared/model";
import { OsSplitView } from "../window-tabs-shared/os-split-view";
import { OsTabBar } from "../window-tabs-shared/os-tab-bar";
import { WindowShell } from "../window-tabs-shared/shell";
import { DesktopIndicators, type DesktopIndicatorsProps } from "./indicators";
import { DesktopsOverviewDraft, type DesktopsOverviewDraftProps } from "./overview";

export const meta = definePrototype({
  title: "Desktops overview and indicators",
  spec: "window-tabs-split",
  surface: "S8 S9",
  route: "/",
  states: [
    { id: "overview-many", label: "S8 · Overview" },
    { id: "overview-one", label: "S8 · One desktop" },
    { id: "overview-full", label: "S8 · A desktop with many tabs" },
    { id: "overview-rename", label: "S8 · Renaming a desktop" },
    { id: "overview-reorder", label: "S8 · Dragging a card" },
    { id: "overview-drop-tab", label: "S8 · Tab dragged onto a card" },
    { id: "overview-close-pick", label: "S8 · Closing: choose where tabs go" },
    { id: "overview-close-ready", label: "S8 · Closing: ready to confirm" },
    { id: "overview-close-done", label: "S8 · Closing: confirmed" },
    { id: "indicator-rest", label: "S9 · Indicators" },
    { id: "indicator-menu", label: "S9 · Indicator menu" },
    { id: "indicator-menu-first", label: "S9 · Menu on the first desktop" },
    { id: "indicator-targets", label: "S9 · Dragging a tab: drop targets" },
    { id: "indicator-drop", label: "S9 · Tab over a desktop" },
    { id: "indicator-overflow", label: "S9 · More desktops than fit" },
    { id: "indicator-overflow-drop", label: "S9 · Tab over the overflow" },
  ],
});

const MORE: readonly DraftDesktop[] = [
  ...DESKTOPS,
  { id: "docs", name: "Docs", tabs: [TAB.docs, TAB.files], activeTabId: TAB.docs.id },
  { id: "ops", name: "Ops", tabs: [TAB.term, TAB.twoUp], activeTabId: TAB.twoUp.id },
];

const FULL: readonly DraftDesktop[] = DESKTOPS.map(desktop =>
  desktop.id === "build"
    ? { ...desktop, tabs: [...desktop.tabs, TAB.flaky, TAB.term, TAB.files, TAB.threeWay] }
    : desktop
);

const BUILD = DESKTOPS.find(desktop => desktop.id === "build")!;

const OVERVIEWS: Record<string, DesktopsOverviewDraftProps> = {
  "overview-many": { desktops: MORE, activeDesktopId: "build" },
  "overview-one": { desktops: [BUILD], activeDesktopId: "build" },
  "overview-full": { desktops: FULL, activeDesktopId: "build" },
  "overview-rename": {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    cards: { review: { renaming: true } },
  },
  "overview-reorder": {
    desktops: [DESKTOPS[0]!, { ...DESKTOPS[2]!, id: "gap" }, DESKTOPS[1]!, DESKTOPS[2]!],
    activeDesktopId: "build",
    cards: { gap: { placeholder: true }, review: { lifted: true } },
  },
  "overview-drop-tab": {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    cards: { review: { dropTarget: true } },
    draggedTab: { tab: TAB.twoUp, left: "71%", top: "52%" },
  },
  "overview-close-pick": {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    cards: { review: { closing: { phase: "open" } } },
  },
  "overview-close-ready": {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    cards: { review: { closing: { phase: "open", destinationId: "build" } } },
  },
  "overview-close-done": {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    cards: { review: { closing: { phase: "confirmed", destinationId: "build" } } },
  },
};

const MANY_DESKTOPS: readonly DraftDesktop[] = [
  ...MORE,
  { id: "lab", name: "Lab", tabs: [TAB.auth], activeTabId: TAB.auth.id },
  { id: "notes", name: "Notes", tabs: [TAB.docs], activeTabId: TAB.docs.id },
];

const INDICATORS: Record<string, DesktopIndicatorsProps> = {
  "indicator-rest": { desktops: DESKTOPS, activeDesktopId: "build" },
  "indicator-menu": { desktops: DESKTOPS, activeDesktopId: "build", menuId: "build" },
  "indicator-menu-first": { desktops: DESKTOPS, activeDesktopId: "build", menuId: "control" },
  "indicator-targets": { desktops: DESKTOPS, activeDesktopId: "build", targets: true },
  "indicator-drop": {
    desktops: DESKTOPS,
    activeDesktopId: "build",
    targets: true,
    hoverId: "review",
  },
  "indicator-overflow": {
    desktops: MANY_DESKTOPS.slice(0, 5),
    activeDesktopId: "build",
    hiddenCount: 2,
  },
  "indicator-overflow-drop": {
    desktops: MANY_DESKTOPS.slice(0, 5),
    activeDesktopId: "build",
    targets: true,
    hiddenCount: 2,
    overflowHover: true,
  },
};

const TABS = BUILD.tabs;

export default function WindowDesktopsDraft({ state }: PrototypeProps) {
  const overview = OVERVIEWS[state];
  const indicators = INDICATORS[state];
  const dragging = indicators?.targets === true;
  return (
    <WindowShell
      bar={
        <OsTabBar
          activeTabId={TAB.twoUp.id}
          tabProps={dragging ? { [TAB.review.id]: { dragging: true } } : undefined}
          tabs={TABS}
          windows={WINDOWS}
        />
      }
      overlay={
        <>
          {overview ? <DesktopsOverviewDraft {...overview} /> : null}
          {dragging ? (
            <span className="pointer-events-none absolute top-9 right-44 z-50">
              <DragGhost source={{ kind: "tab", tab: TAB.review }} windows={WINDOWS} x={0} y={0} />
            </span>
          ) : null}
        </>
      }
      pager={indicators ? <DesktopIndicators {...indicators} /> : undefined}
    >
      <OsSplitView focusedId="tabs" root={TAB.twoUp.root} windows={WINDOWS} />
    </WindowShell>
  );
}
