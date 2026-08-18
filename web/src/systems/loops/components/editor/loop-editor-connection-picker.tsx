import { useState, type KeyboardEvent } from "react";

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Eyebrow,
  Kbd,
  KindIcon,
  Popover,
  PopoverContent,
  type KindIconRegistry,
} from "@compozy/ui";

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

const PICKER_KIND_ICONS = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

export interface LoopEditorConnectionPickerProps {
  open: boolean;

  point: { x: number; y: number } | null;
  sourceNodeId: string;
  onOpenChange: (open: boolean) => void;

  onPick: (item: PaletteItem) => void;
}

type ConnectionPickerPanelProps = Omit<LoopEditorConnectionPickerProps, "open" | "point">;

function paletteGlyph(item: PaletteItem) {
  return loopNodeClassIcon({
    nodeClass: item.nodeClass,
    isFanOut: item.kindLabel === "fan-out",
    isGate: item.kindLabel === "gate",
  });
}

function ConnectionPickerPanel({ sourceNodeId, onOpenChange, onPick }: ConnectionPickerPanelProps) {
  const [query, setQuery] = useState("");
  const groups: { label: string; items: PaletteItem[] }[] = [];
  for (const group of LOOP_PALETTE) {
    const items = filterPaletteItems(group.items, query);
    if (items.length > 0) groups.push({ label: group.label, items });
  }

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Escape") return;

    event.preventDefault();
    onOpenChange(false);
  };

  return (
    <Command
      aria-label={`Add a node and wire it from ${sourceNodeId}`}
      className="bg-transparent p-0 shadow-none"
      data-testid="loop-editor-connection-picker"
      onKeyDown={handleKeyDown}
      shouldFilter={false}
    >
      <div className="flex items-center gap-2 px-2.5 pt-2 pb-0.5">
        <Eyebrow className="shrink-0 text-muted">Add and wire</Eyebrow>
        <span
          className="min-w-0 truncate font-mono text-badge tracking-mono text-faint"
          title={sourceNodeId}
        >
          {sourceNodeId}
        </span>
      </div>
      <CommandInput autoFocus onValueChange={setQuery} placeholder="Filter kinds…" value={query} />
      <CommandList className="max-h-56">
        <CommandEmpty>No kinds match “{query.trim()}”.</CommandEmpty>
        {groups.map(group => (
          <CommandGroup heading={group.label} key={group.label}>
            {group.items.map(item => (
              <CommandItem
                data-testid={`loop-connection-picker-item-${item.kindLabel}`}
                key={item.kindLabel}
                onSelect={() => {
                  onPick(item);
                  onOpenChange(false);
                }}
                value={item.label}
              >
                <KindIcon
                  className="size-3.5"
                  fallback={paletteGlyph(item)}
                  kind={paletteKindKey(item.kindLabel)}
                  registry={PICKER_KIND_ICONS}
                  size="xs"
                  tone="muted"
                />
                <span className="min-w-0 truncate">{item.label}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        ))}
      </CommandList>
      <div className="flex items-center gap-4 border-t border-line px-2.5 py-2 text-micro text-subtle">
        <span className="flex items-center gap-1.5">
          <Kbd>↑↓</Kbd>move
        </span>
        <span className="flex items-center gap-1.5">
          <Kbd>⏎</Kbd>add
        </span>
        <span className="ml-auto flex items-center gap-1.5">
          <Kbd>esc</Kbd>cancel
        </span>
      </div>
    </Command>
  );
}

export function LoopEditorConnectionPicker({
  open,
  point,
  sourceNodeId,
  onOpenChange,
  onPick,
}: LoopEditorConnectionPickerProps) {
  const x = point?.x;
  const y = point?.y;

  const anchor =
    x === undefined || y === undefined
      ? undefined
      : { getBoundingClientRect: () => new DOMRect(x, y, 0, 0) };
  if (anchor === undefined) return null;
  return (
    <Popover onOpenChange={onOpenChange} open={open}>
      <PopoverContent
        align="start"
        anchor={anchor}
        className="w-56 gap-0 p-0"
        side="bottom"
        sideOffset={6}
      >
        <ConnectionPickerPanel
          onOpenChange={onOpenChange}
          onPick={onPick}
          sourceNodeId={sourceNodeId}
        />
      </PopoverContent>
    </Popover>
  );
}
