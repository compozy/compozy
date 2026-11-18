import { Pin } from "@compozy/ui/icons";

import { LaneTabs } from "@compozy/ui";

import { layoutNodeWindowIds } from "../../lib/window-manager-layout-tree";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutTab,
} from "../../lib/window-manager-layout-types";

interface LayoutTabStripProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  selectedTabId: string;
  onSelect: (tabId: string) => void;
}

function TabLabel({
  document,
  desktop,
  tab,
}: {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  tab: WindowManagerLayoutTab;
}) {
  const lead = layoutNodeWindowIds(tab.root)[0] ?? "";
  const face = layoutWindowFace(document.windows[lead], lead);
  const Icon = face.icon;
  return (
    <span className="inline-flex max-w-56 min-w-0 items-center gap-1.5">
      {tab.pinned ? <Pin aria-label="Pinned" className="size-3 text-subtle" /> : null}
      <Icon aria-hidden="true" className="size-3.5 text-subtle" />
      <span className="truncate">{face.title}</span>
      {tab.id === desktop.activeTabId ? (
        <span className="text-badge font-normal text-faint">Opens here</span>
      ) : null}
    </span>
  );
}

/**
 * The desktop's tab bar as the document has it: pinned tabs first, each named
 * for its first window, with the member count of a split. The tab the desktop
 * opens on is marked; picking a tab here only puts it on the canvas.
 */
export function LayoutTabStrip({
  document,
  desktop,
  selectedTabId,
  onSelect,
}: LayoutTabStripProps) {
  return (
    <LaneTabs
      ariaLabel="Tabs"
      className="min-w-0 border-b border-line-soft px-2.5"
      data-testid="layout-tab-strip"
      items={desktop.tabs.map(tab => {
        const members = layoutNodeWindowIds(tab.root).length;
        return {
          value: tab.id,
          label: <TabLabel desktop={desktop} document={document} tab={tab} />,
          count: members > 1 ? members : undefined,
          testId: `layout-tab-${tab.id}`,
        };
      })}
      listClassName="overflow-x-auto"
      value={selectedTabId}
      onChange={onSelect}
    />
  );
}
