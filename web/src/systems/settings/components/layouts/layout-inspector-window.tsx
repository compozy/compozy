import { useState } from "react";

import { Button, Eyebrow, Item, ItemContent, ItemMedia, ItemTitle, PillGroup } from "@compozy/ui";

import {
  closeLayoutWindow,
  createLayoutIdFactory,
  locateLayoutWindow,
  moveLayoutWindowBeside,
  moveLayoutWindowToNewTab,
  swapLayoutWindowPlaces,
} from "../../lib/window-manager-layout-mutations";
import { layoutWindowFace } from "../../lib/window-manager-layout-window-face";
import type {
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDocument,
  WindowManagerLayoutEdge,
  WindowManagerLayoutTab,
} from "../../lib/window-manager-layout-types";

interface LayoutInspectorWindowProps {
  document: WindowManagerLayoutDocument;
  desktop: WindowManagerLayoutDesktop;
  tab: WindowManagerLayoutTab;
  windowId: string;
  /** Receives the edited document; a refused edit returns the same document and is ignored. */
  onChange: (next: WindowManagerLayoutDocument) => void;
  onReveal: (tabId: string, windowId: string | null) => void;
}

const EDGES: ReadonlyArray<{ value: WindowManagerLayoutEdge; label: string }> = [
  { value: "left", label: "Left" },
  { value: "right", label: "Right" },
  { value: "top", label: "Above" },
  { value: "bottom", label: "Below" },
];

/**
 * What the selected window can do in the layout. Every window appears exactly
 * once, so assigning a window swaps two places instead of copying one, and
 * splitting moves a window from wherever it is; each action is a pure edit
 * that leaves the document valid or changes nothing.
 */
export function LayoutInspectorWindow({
  document,
  desktop,
  tab,
  windowId,
  onChange,
  onReveal,
}: LayoutInspectorWindowProps) {
  const [edge, setEdge] = useState<WindowManagerLayoutEdge>("right");
  const others = desktop.tabs
    .flatMap(candidate => (candidate.id === tab.id ? [] : collectWindows(candidate)))
    .concat(collectWindows(tab).filter(id => id !== windowId));
  const inSplit = tab.root.kind === "split";
  const lastWindow = !inSplit && desktop.tabs.length < 2;

  /** Commits an edit and keeps the selected window on screen wherever it moved. */
  const commit = (next: WindowManagerLayoutDocument) => {
    if (next === document) return;
    onChange(next);
    const location = locateLayoutWindow(next, windowId);
    onReveal(location?.tab.id ?? tab.id, location ? windowId : null);
  };

  return (
    <>
      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Swap with</Eyebrow>
        <WindowChoiceList
          document={document}
          emptyLabel="No other window on this desktop."
          testIdPrefix="layout-inspector-swap"
          windowIds={others}
          onChoose={otherId => commit(swapLayoutWindowPlaces(document, windowId, otherId))}
        />
      </section>

      <section className="flex flex-col gap-2">
        <Eyebrow className="text-faint">Bring a window beside it</Eyebrow>
        <PillGroup
          aria-label="Side"
          items={EDGES.map(entry => ({ value: entry.value, label: entry.label }))}
          size="sm"
          value={edge}
          onChange={setEdge}
        />
        <WindowChoiceList
          document={document}
          emptyLabel="No other window on this desktop."
          testIdPrefix="layout-inspector-beside"
          windowIds={others}
          onChoose={otherId =>
            commit(
              moveLayoutWindowBeside(
                document,
                otherId,
                windowId,
                edge,
                createLayoutIdFactory(document)
              )
            )
          }
        />
      </section>

      <section className="flex flex-wrap gap-1.5">
        {inSplit ? (
          <Button
            data-testid="layout-inspector-new-tab"
            size="xs"
            type="button"
            variant="secondary"
            onClick={() =>
              commit(moveLayoutWindowToNewTab(document, windowId, createLayoutIdFactory(document)))
            }
          >
            Move to new tab
          </Button>
        ) : null}
        <Button
          data-testid="layout-inspector-remove-window"
          disabled={lastWindow}
          size="xs"
          type="button"
          variant="ghost"
          onClick={() => commit(closeLayoutWindow(document, windowId))}
        >
          Remove from layout
        </Button>
      </section>
      {lastWindow ? (
        <p className="text-form-hint text-subtle">
          A desktop keeps at least one tab, so its last window stays.
        </p>
      ) : null}
    </>
  );
}

function collectWindows(tab: WindowManagerLayoutTab): string[] {
  const ids: string[] = [];
  const pending = [tab.root];
  for (let node = pending.shift(); node !== undefined; node = pending.shift()) {
    if (node.kind === "leaf") ids.push(node.windowId);
    else pending.push(...node.children);
  }
  return ids;
}

function WindowChoiceList({
  document,
  windowIds,
  emptyLabel,
  testIdPrefix,
  onChoose,
}: {
  document: WindowManagerLayoutDocument;
  windowIds: readonly string[];
  emptyLabel: string;
  testIdPrefix: string;
  onChoose: (windowId: string) => void;
}) {
  if (windowIds.length === 0) return <p className="text-form-hint text-subtle">{emptyLabel}</p>;

  return (
    <div className="flex flex-col gap-0.5">
      {windowIds.map(windowId => {
        const face = layoutWindowFace(document.windows[windowId], windowId);
        const Icon = face.icon;
        return (
          <Item
            as="button"
            className="py-1.5"
            data-testid={`${testIdPrefix}-${windowId}`}
            indicator="none"
            key={windowId}
            selectable
            size="xs"
            type="button"
            onClick={() => onChoose(windowId)}
          >
            <ItemMedia>
              <Icon aria-hidden="true" className="size-3.5 text-subtle" />
            </ItemMedia>
            <ItemContent className="min-w-0 gap-0">
              <ItemTitle className="truncate">{face.title}</ItemTitle>
            </ItemContent>
            <span className="shrink-0 truncate font-mono text-badge text-faint">{face.detail}</span>
          </Item>
        );
      })}
    </div>
  );
}
