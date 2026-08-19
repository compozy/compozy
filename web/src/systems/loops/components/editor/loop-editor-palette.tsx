import { useId, useState, type KeyboardEvent } from "react";

import { cn, Eyebrow, KindIcon, SearchInput, type KindIconRegistry } from "@compozy/ui";

import {
  LOOP_CALL_TOOL_ICON,
  LOOP_NODE_KIND_ICONS,
  loopNodeClassIcon,
} from "../../lib/loop-node-kind-icons";
import {
  filterPaletteItems,
  LOOP_PALETTE,
  paletteKindKey,
  type PaletteItem,
} from "../../lib/loop-palette";
import { MonoTag } from "../mono-tag";

interface LoopEditorPaletteProps {
  onAddNode: (item: PaletteItem) => void;
  disabled?: boolean;
}

const LOOP_EDITOR_KIND_ICON_REGISTRY = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

const PALETTE_ITEM_CLASS = [
  "group flex items-center gap-2 rounded-md border px-2 py-1.5 text-left transition-colors",
  "hover:border-line-strong hover:bg-canvas-tint",
  "disabled:cursor-not-allowed disabled:opacity-60",
];

function optionId(listId: string, item: PaletteItem): string {
  return `${listId}-${item.kindLabel}`;
}

export function LoopEditorPalette({ onAddNode, disabled = false }: LoopEditorPaletteProps) {
  const listId = useId();
  const [query, setQuery] = useState("");
  const [activeIndex, setActiveIndex] = useState(0);
  const groups: { label: string; items: PaletteItem[] }[] = [];
  for (const group of LOOP_PALETTE) {
    const items = filterPaletteItems(group.items, query);
    if (items.length > 0) groups.push({ label: group.label, items });
  }
  const matches = groups.flatMap(group => group.items);

  const activeItem = matches[Math.min(activeIndex, matches.length - 1)] ?? null;

  const addItem = (item: PaletteItem) => {
    if (disabled) return;
    onAddNode(item);
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex(index => Math.min(index + 1, Math.max(matches.length - 1, 0)));
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex(index => Math.max(index - 1, 0));
      return;
    }
    if (event.key === "Enter") {
      event.preventDefault();
      if (activeItem) addItem(activeItem);
      return;
    }
    if (event.key === "Escape" && query !== "") {
      event.preventDefault();
      setQuery("");
      setActiveIndex(0);
    }
  };

  return (
    <aside
      className="flex min-h-0 flex-col gap-3 overflow-y-auto border-r border-line bg-canvas px-3 py-3.5"
      data-testid="loop-editor-palette"
    >
      <Eyebrow className="text-subtle">Add node</Eyebrow>
      <SearchInput
        aria-activedescendant={activeItem ? optionId(listId, activeItem) : undefined}
        aria-controls={listId}
        aria-label="Filter node kinds"
        containerClassName="min-w-0"
        data-testid="loop-palette-search"
        disabled={disabled}
        onChange={next => {
          setQuery(next);
          setActiveIndex(0);
        }}
        onKeyDown={handleKeyDown}
        placeholder="Filter kinds…"
        value={query}
      />
      {matches.length === 0 ? (
        <p className="px-0.5 text-form-hint text-subtle" data-testid="loop-palette-empty">
          No kinds match “{query.trim()}”.
        </p>
      ) : (
        <div aria-label="Node kinds" className="flex flex-col gap-4" id={listId} role="listbox">
          {groups.map(group => (
            <div
              aria-label={group.label}
              className="flex flex-col gap-1.5"
              key={group.label}
              role="group"
            >
              <MonoTag className="px-0.5 text-pill-group-badge tracking-[0.09em] text-faint">
                {group.label}
              </MonoTag>
              {group.items.map(item => {
                const active = item === activeItem;
                return (
                  <button
                    aria-selected={active}
                    className={cn(
                      PALETTE_ITEM_CLASS,
                      active
                        ? "border-line-strong bg-canvas-tint"
                        : "border-line-soft bg-canvas-soft"
                    )}
                    data-testid={`loop-palette-item-${item.kindLabel}`}
                    disabled={disabled}
                    id={optionId(listId, item)}
                    key={item.label}
                    onClick={() => addItem(item)}
                    role="option"
                    title={item.hint}
                    type="button"
                  >
                    <span className="grid size-4 shrink-0 place-items-center rounded-xs bg-badge-fill transition-transform group-hover:scale-110">
                      <KindIcon
                        className="size-2.5"
                        fallback={loopNodeClassIcon({
                          nodeClass: item.nodeClass,
                          isFanOut: item.kindLabel === "fan-out",
                          isGate: item.kindLabel === "gate",
                        })}
                        kind={paletteKindKey(item.kindLabel)}
                        registry={LOOP_EDITOR_KIND_ICON_REGISTRY}
                        size="xs"
                        tone="muted"
                      />
                    </span>
                    <span className="truncate text-form-label font-medium text-fg-strong">
                      {item.label}
                    </span>
                  </button>
                );
              })}
            </div>
          ))}
        </div>
      )}
    </aside>
  );
}
