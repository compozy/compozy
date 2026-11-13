import type { CSSProperties } from "react";

import { cn } from "@compozy/ui";

import { DragGhost } from "../window-tabs-shared/drag-ghost";
import { WINDOWS } from "../window-tabs-shared/fixtures";
import { OsSplitView } from "../window-tabs-shared/os-split-view";
import { OsTabBar } from "../window-tabs-shared/os-tab-bar";
import { WindowShell } from "../window-tabs-shared/shell";
import { UpgradeNotice } from "../window-tabs-shared/upgrade-notice";
import type { Scene } from "./scenes";

export function SceneView({ scene }: { scene: Scene }) {
  const active = scene.tabs.find(tab => tab.id === scene.activeTabId) ?? scene.tabs[0]!;
  const content = scene.contentTab ?? active;
  const ghost = scene.ghost;
  const ghostStyle: CSSProperties | undefined = ghost
    ? { left: ghost.left, top: ghost.top }
    : undefined;
  const ghostNode = ghost ? (
    <span className="pointer-events-none absolute z-50" style={ghostStyle}>
      <DragGhost refused={ghost.refused} source={ghost.source} windows={WINDOWS} x={0} y={0} />
    </span>
  ) : null;
  const shell = (
    <WindowShell
      bar={
        <OsTabBar
          activeTabId={scene.activeTabId}
          compact={scene.compact}
          insertionIndex={scene.insertionIndex}
          overflowing={scene.overflowing}
          tabProps={scene.tabProps}
          tabs={scene.tabs}
          windows={WINDOWS}
        />
      }
      className={cn(scene.compact && "h-full")}
      compact={scene.compact}
      notice={scene.notice ? <UpgradeNotice state={scene.notice} /> : null}
      overlay={ghost?.layer === "shell" ? ghostNode : null}
    >
      <OsSplitView
        draggingId={scene.draggingId}
        drop={scene.drop}
        focusedId={scene.focusedId}
        root={content.root}
        rootDrop={scene.rootDrop}
        seams={scene.seams}
        stacked={scene.stacked}
        windows={WINDOWS}
      >
        {ghost?.layer === "content" ? ghostNode : null}
      </OsSplitView>
    </WindowShell>
  );
  if (!scene.compact) return shell;
  return (
    <div className="grid h-dvh place-items-center bg-sunken">
      <div className="h-[720px] w-[640px] overflow-hidden rounded-lg shadow-elevated">{shell}</div>
    </div>
  );
}
