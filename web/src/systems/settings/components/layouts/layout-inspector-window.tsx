import { Check } from "lucide-react";

import { Eyebrow, Item, ItemContent, ItemMedia, ItemTitle, cn } from "@agh/ui";

import {
  desktopWindowIds,
  layoutWindowOwners,
  replaceLayoutNode,
} from "../../lib/window-manager-layout-tree";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
} from "../../lib/window-manager-layout-types";

interface LayoutInspectorWindowProps {
  node: WindowManagerLayoutNode;
  desktop: WindowManagerLayoutDesktop;
  document: WindowManagerLayoutDocument;
  onDesktopChange: (next: WindowManagerLayoutDesktop) => void;
}

/**
 * What this tile shows. A leaf picks from the desktop's windows, with the ones
 * another node already holds ruled out — the daemon rejects a document that
 * places the same window twice, so the picker refuses it rather than reporting
 * it afterwards. A stack picks which of its members is on top.
 */
export function LayoutInspectorWindow({
  node,
  desktop,
  document,
  onDesktopChange,
}: LayoutInspectorWindowProps) {
  if (node.kind === "leaf") {
    const owners = layoutWindowOwners(desktop);
    return (
      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Shows</Eyebrow>
        <WindowChoiceList
          document={document}
          selectedId={node.windowId}
          unavailable={windowId => {
            const owner = owners.get(windowId);
            return owner !== undefined && owner !== node.id;
          }}
          windowIds={desktopWindowIds(document, desktop.id)}
          onChoose={windowId =>
            onDesktopChange(replaceLayoutNode(desktop, node.id, { ...node, windowId }))
          }
        />
      </section>
    );
  }

  if (node.kind !== "stack") return null;

  return (
    <section className="flex flex-col gap-2">
      <Eyebrow className="text-faint">On top</Eyebrow>
      <WindowChoiceList
        document={document}
        selectedId={node.activeId}
        unavailable={() => false}
        windowIds={node.windowIds}
        onChoose={activeId =>
          onDesktopChange(replaceLayoutNode(desktop, node.id, { ...node, activeId }))
        }
      />
      {node.windowIds.length < 2 ? (
        <p className="text-form-hint text-warning">
          A stack needs at least two windows. The daemon refuses one that holds a single window.
        </p>
      ) : null}
    </section>
  );
}

function WindowChoiceList({
  document,
  windowIds,
  selectedId,
  unavailable,
  onChoose,
}: {
  document: WindowManagerLayoutDocument;
  windowIds: readonly string[];
  selectedId: string;
  unavailable: (windowId: string) => boolean;
  onChoose: (windowId: string) => void;
}) {
  if (windowIds.length === 0) {
    return <p className="text-form-hint text-subtle">This desktop has no windows to show.</p>;
  }

  return (
    <div className="flex flex-col gap-0.5" role="radiogroup">
      {windowIds.map(windowId => {
        const face = layoutWindowFace(document.windows[windowId], windowId);
        const Icon = face.icon;
        const taken = unavailable(windowId);
        const selected = windowId === selectedId;
        return (
          <Item
            aria-checked={selected}
            as="button"
            className={cn("py-1.5", taken && "cursor-not-allowed opacity-45")}
            data-testid={`layout-inspector-window-${windowId}`}
            disabled={taken}
            indicator="none"
            key={windowId}
            role="radio"
            selectable
            selected={selected}
            size="xs"
            type="button"
            onClick={() => {
              if (!taken) onChoose(windowId);
            }}
          >
            <ItemMedia>
              <Icon aria-hidden="true" className="size-3.5 text-subtle" />
            </ItemMedia>
            <ItemContent className="min-w-0 gap-0">
              <ItemTitle className="truncate">{face.title}</ItemTitle>
            </ItemContent>
            {selected ? (
              <Check aria-hidden="true" className="size-3 shrink-0 text-accent-strong" />
            ) : (
              <span className="shrink-0 font-mono text-badge text-faint">
                {taken ? "in use" : face.detail}
              </span>
            )}
          </Item>
        );
      })}
    </div>
  );
}
