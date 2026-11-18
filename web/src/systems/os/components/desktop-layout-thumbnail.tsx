import { cn } from "@compozy/ui";

import { getOsAppDescriptor } from "../lib/app-catalog";
import type { OsAppId } from "../lib/os-types";
import type { LayoutNode, TabId, WindowId } from "../lib/window-manager-types";

/** Tab marks drawn on the thumbnail's strip before it stops adding more. */
const STRIP_TABS = 8;

function ThumbnailNode({
  node,
  appOf,
}: {
  node: LayoutNode;
  appOf: (windowId: WindowId) => OsAppId | undefined;
}) {
  if (node.kind === "leaf") {
    const app = appOf(node.windowId);
    const Icon = app ? getOsAppDescriptor(app).icon : null;
    return (
      <span
        className="relative grid min-h-0 min-w-0 flex-1 place-items-center overflow-hidden rounded-xxs bg-elevated shadow-card"
        data-slot="desktop-thumbnail-window"
        data-window-id={node.windowId}
      >
        <span className="absolute inset-x-0 top-0 h-1.75 border-b border-line-soft bg-surface-2" />
        {Icon ? <Icon aria-hidden="true" className="mt-1.5 size-4 text-subtle" /> : null}
      </span>
    );
  }
  return (
    <span
      className={cn(
        "flex min-h-0 min-w-0 flex-1 gap-0.5",
        node.axis === "horizontal" ? "flex-row" : "flex-col"
      )}
    >
      {node.children.map((child, index) => (
        <span
          className="flex min-h-0 min-w-0"
          key={child.id}
          style={{ flex: `${node.weights[index] ?? 1} 1 0` }}
        >
          <ThumbnailNode appOf={appOf} node={child} />
        </span>
      ))}
    </span>
  );
}

export interface DesktopLayoutThumbnailProps {
  /** The desktop's tabs in bar order, drawn as the strip's marks. */
  tabIds: readonly TabId[];
  activeTabId: TabId | null;
  /** The active tab's split tree; each window takes its share by weight. */
  root: LayoutNode | null;
  appOf: (windowId: WindowId) => OsAppId | undefined;
}

/**
 * A miniature of one desktop: its tab strip, then its active tab's windows laid out from the
 * split tree's weights, each with its app's icon. Presentational only.
 */
export function DesktopLayoutThumbnail({
  tabIds,
  activeTabId,
  root,
  appOf,
}: DesktopLayoutThumbnailProps) {
  return (
    <span
      aria-hidden="true"
      className="flex aspect-16/10 w-full flex-col gap-1 rounded-md bg-rail p-1.5"
      data-slot="desktop-thumbnail"
    >
      <span className="flex h-2 shrink-0 items-center gap-0.5">
        {tabIds.slice(0, STRIP_TABS).map(tabId => (
          <span
            className={cn(
              "h-full max-w-8 flex-1 rounded-xxs",
              tabId === activeTabId ? "bg-fg/25" : "bg-fg/8"
            )}
            key={tabId}
          />
        ))}
      </span>
      {root ? <ThumbnailNode appOf={appOf} node={root} /> : null}
    </span>
  );
}
