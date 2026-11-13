import { Pin } from "@compozy/ui/icons";

import { cn } from "@compozy/ui";

import { layoutNodeWindowIds } from "../../lib/window-manager-layout-tree";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
} from "../../lib/window-manager-layout-types";

interface LayoutTabStripProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  selectedTabId: string | null;
  onSelect: (tabId: string) => void;
}

/**
 * The desktop's tab bar as the document has it: pinned tabs first, each named
 * for its first window with the member count of a split. The tab the desktop
 * opens on is marked; picking a tab here only puts it on the canvas.
 */
export function LayoutTabStrip({
  document,
  desktop,
  selectedTabId,
  onSelect,
}: LayoutTabStripProps) {
  return (
    <div
      aria-label="Tabs"
      className="flex min-w-0 items-center gap-1 overflow-x-auto border-b border-line-soft px-2.5 py-1.5"
      data-testid="layout-tab-strip"
      role="tablist"
    >
      {desktop.tabs.map(tab => {
        const windowIds = layoutNodeWindowIds(tab.root);
        const lead = windowIds[0] ?? "";
        const face = layoutWindowFace(document.windows[lead], lead);
        const Icon = face.icon;
        const selected = tab.id === selectedTabId;
        return (
          <button
            aria-selected={selected}
            className={cn(
              "inline-flex h-control-compact max-w-56 shrink-0 items-center gap-1.5 rounded-md px-2",
              "text-small-body text-muted-foreground transition-colors duration-base ease-out",
              "hover:bg-surface-2 hover:text-fg",
              "focus-visible:outline-none focus-visible:shadow-focus-ring",
              selected && "bg-surface-2 text-fg"
            )}
            data-pinned={tab.pinned ? "true" : undefined}
            data-testid={`layout-tab-${tab.id}`}
            key={tab.id}
            role="tab"
            type="button"
            onClick={() => onSelect(tab.id)}
          >
            {tab.pinned ? (
              <Pin aria-label="Pinned" className="size-3 shrink-0 text-subtle" />
            ) : null}
            <Icon aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
            <span className="truncate">{face.title}</span>
            {windowIds.length > 1 ? (
              <span className="shrink-0 font-mono text-badge text-faint">{windowIds.length}</span>
            ) : null}
            {tab.id === desktop.activeTabId ? (
              <span className="shrink-0 text-badge text-faint">Opens here</span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
