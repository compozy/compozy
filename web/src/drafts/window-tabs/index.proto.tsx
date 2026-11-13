import { definePrototype, type PrototypeProps } from "@compozy/proto";

import { LivePlayground } from "./live";
import { SceneView } from "./scene-view";
import { SCENES } from "./scenes";
import { TabSheet } from "./tab-sheet";

export const meta = definePrototype({
  title: "Window tabs and splits",
  spec: "window-tabs-split",
  surface: "S1 S2 S3 S4 S5 S6 S10",
  route: "/",
  states: [
    { id: "live", label: "Try it · drag tabs and windows" },
    { id: "split-two", label: "S3 · Two windows side by side" },
    { id: "split-single", label: "S3 · One window" },
    { id: "split-three", label: "S3 · Three windows, nested" },
    { id: "split-four", label: "S3 · Four windows" },
    { id: "split-stacked", label: "S3 · No room: one window at a time" },
    { id: "bar-one", label: "S1 · One tab" },
    { id: "bar-pinned", label: "S1 · Pinned and regular tabs" },
    { id: "bar-many", label: "S1 · Many tabs, shrunk" },
    { id: "bar-crowded", label: "S1 · Crowded: icons only" },
    { id: "bar-overflow", label: "S1 · Too many tabs: scroll and list" },
    { id: "bar-reorder", label: "S1 · Reordering a tab" },
    { id: "bar-insertion", label: "S1 · Window dragged to the bar" },
    { id: "bar-dwell", label: "S1 · Window held over another tab" },
    { id: "bar-compact", label: "S1 · Small screen" },
    { id: "tab-sheet", label: "S2 · Every tab variant" },
    { id: "drop-right", label: "S5 · Tab dropped on the right" },
    { id: "drop-bottom", label: "S5 · Tab dropped at the bottom" },
    { id: "drop-nested", label: "S5 · Tab dropped inside a split" },
    { id: "drop-root", label: "S5 · Tab dropped at the outer edge" },
    { id: "drop-self", label: "S5 · Dragging the tab on screen" },
    { id: "drop-head", label: "S5 · Window moved to another edge" },
    { id: "drop-swap", label: "S5 · Swapping two windows" },
    { id: "drop-withheld", label: "S5 · No room to drop here" },
    { id: "seam-hover", label: "S6 · Divider, hover" },
    { id: "seam-drag", label: "S6 · Divider, dragging" },
    { id: "seam-focus", label: "S6 · Divider, keyboard" },
    { id: "seam-limit", label: "S6 · Divider at the minimum size" },
    { id: "notice-migrated", label: "S10 · After the upgrade" },
    { id: "notice-failed", label: "S10 · Upgrade could not move the layout" },
    { id: "notice-retrying", label: "S10 · Retrying the upgrade" },
    { id: "notice-layout-changed", label: "S10 · Layout changed since the failure" },
    { id: "notice-replace-confirm", label: "S10 · Confirm replacing the layout" },
  ],
});

export default function WindowTabsDraft({ state }: PrototypeProps) {
  if (state === "tab-sheet") return <TabSheet />;
  const scene = SCENES[state];
  if (!scene) return <LivePlayground />;
  return <SceneView scene={scene} />;
}
