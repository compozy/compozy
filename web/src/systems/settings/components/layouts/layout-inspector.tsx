import { ArrowLeft, ArrowRight, Pin, PinOff, X } from "@compozy/ui/icons";

import { Button, Eyebrow, PropertyRow } from "@compozy/ui";

import {
  closeLayoutTab,
  moveLayoutTab,
  replaceLayoutTabRoot,
  setLayoutDefaultTab,
  setLayoutTabPinned,
} from "../../lib/window-manager-layout-mutations";
import {
  findLayoutLeaf,
  findLayoutParentSplit,
  layoutNodeWindowIds,
} from "../../lib/window-manager-layout-tree";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutTab,
} from "../../lib/window-manager-layout-types";
import { LayoutInspectorSplit } from "./layout-inspector-node";
import { LayoutInspectorWindow } from "./layout-inspector-window";

interface LayoutInspectorProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  tab: WindowManagerLayoutTab;
  windowId: string | null;
  onChange: (next: WindowManagerLayoutDocument) => void;
  onReveal: (tabId: string, windowId: string | null) => void;
}

/** The tab on the canvas, and — when one is picked — a window of it and the split it sits in. */
export function LayoutInspector({
  document,
  desktop,
  tab,
  windowId,
  onChange,
  onReveal,
}: LayoutInspectorProps) {
  const leaf = windowId === null ? null : findLayoutLeaf(tab.root, windowId);
  if (leaf === null) {
    return (
      <InspectorShell subtitle={tab.id} title={tabTitle(document, tab)}>
        <LayoutInspectorTab desktop={desktop} document={document} tab={tab} onChange={onChange} />
      </InspectorShell>
    );
  }
  const split = findLayoutParentSplit(tab.root, leaf.id);
  const window = document.windows[leaf.windowId];
  const face = layoutWindowFace(window, leaf.windowId);
  return (
    <InspectorShell subtitle={`${leaf.windowId} · ${face.detail}`} title={face.title}>
      <LayoutInspectorWindow
        desktop={desktop}
        document={document}
        tab={tab}
        windowId={leaf.windowId}
        onChange={onChange}
        onReveal={onReveal}
      />
      {split ? (
        <LayoutInspectorSplit
          root={tab.root}
          split={split}
          onRootChange={root => onChange(replaceLayoutTabRoot(document, desktop.id, tab.id, root))}
        />
      ) : null}
    </InspectorShell>
  );
}

function tabTitle(document: WindowManagerLayoutDocument, tab: WindowManagerLayoutTab): string {
  const lead = layoutNodeWindowIds(tab.root)[0] ?? "";
  return layoutWindowFace(document.windows[lead], lead).title;
}

function LayoutInspectorTab({
  document,
  desktop,
  tab,
  onChange,
}: {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  tab: WindowManagerLayoutTab;
  onChange: (next: WindowManagerLayoutDocument) => void;
}) {
  const windows = layoutNodeWindowIds(tab.root).length;
  const index = desktop.tabs.findIndex(candidate => candidate.id === tab.id);
  const area = desktop.tabs.filter(candidate => candidate.pinned === tab.pinned);
  const areaIndex = area.findIndex(candidate => candidate.id === tab.id);
  const opensHere = desktop.activeTabId === tab.id;
  const commit = (next: WindowManagerLayoutDocument) => {
    if (next !== document) onChange(next);
  };

  return (
    <>
      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Tab</Eyebrow>
        <div>
          <PropertyRow label="Position" mono>
            {`${index + 1} of ${desktop.tabs.length}`}
          </PropertyRow>
          <PropertyRow label="Windows" mono>
            {windows}
          </PropertyRow>
          <PropertyRow label="Pinned">{tab.pinned ? "Yes" : "No"}</PropertyRow>
          <PropertyRow label="Opens on this tab">{opensHere ? "Yes" : "No"}</PropertyRow>
        </div>
        <p className="text-form-label leading-normal text-subtle">
          Click a window to swap it, split beside it or move it to a tab of its own. Drag a line
          between windows to resize them.
        </p>
      </section>

      <section className="flex flex-wrap gap-1.5">
        <Button
          aria-label="Move tab left"
          data-testid="layout-tab-move-left"
          disabled={areaIndex <= 0}
          size="icon-xs"
          type="button"
          variant="secondary"
          onClick={() => commit(moveLayoutTab(document, desktop.id, tab.id, -1))}
        >
          <ArrowLeft aria-hidden="true" className="size-3.5" />
        </Button>
        <Button
          aria-label="Move tab right"
          data-testid="layout-tab-move-right"
          disabled={areaIndex === area.length - 1}
          size="icon-xs"
          type="button"
          variant="secondary"
          onClick={() => commit(moveLayoutTab(document, desktop.id, tab.id, 1))}
        >
          <ArrowRight aria-hidden="true" className="size-3.5" />
        </Button>
        <Button
          data-testid="layout-tab-pin"
          size="xs"
          type="button"
          variant="secondary"
          onClick={() => commit(setLayoutTabPinned(document, desktop.id, tab.id, !tab.pinned))}
        >
          {tab.pinned ? (
            <PinOff aria-hidden="true" className="size-3.5" />
          ) : (
            <Pin aria-hidden="true" className="size-3.5" />
          )}
          {tab.pinned ? "Unpin" : "Pin"}
        </Button>
        <Button
          data-testid="layout-tab-default"
          disabled={opensHere}
          size="xs"
          type="button"
          variant="secondary"
          onClick={() => commit(setLayoutDefaultTab(document, desktop.id, tab.id))}
        >
          Open desktop here
        </Button>
        <Button
          data-testid="layout-tab-close"
          disabled={desktop.tabs.length < 2}
          size="xs"
          type="button"
          variant="ghost"
          onClick={() => commit(closeLayoutTab(document, desktop.id, tab.id))}
        >
          <X aria-hidden="true" className="size-3.5" />
          Close tab
        </Button>
      </section>
      {desktop.tabs.length < 2 ? (
        <p className="text-form-hint text-subtle">A desktop keeps at least one tab.</p>
      ) : null}
    </>
  );
}

function InspectorShell({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle: string;
  children: React.ReactNode;
}) {
  return (
    <aside
      aria-label="Selection inspector"
      className="flex min-w-0 flex-col border-line-soft bg-sunken"
      data-testid="layout-inspector"
    >
      <header className="border-b border-line-soft px-3.5 py-3">
        <h3 className="truncate text-body font-medium text-fg">{title}</h3>
        <p className="mt-0.5 font-mono text-mono-id break-all text-faint">{subtitle}</p>
      </header>
      <div className="flex min-h-0 flex-1 flex-col gap-3.5 overflow-y-auto px-3.5 py-3">
        {children}
      </div>
    </aside>
  );
}
