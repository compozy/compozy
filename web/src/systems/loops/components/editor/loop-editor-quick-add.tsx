import { useState, type KeyboardEvent } from "react";

import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Kbd,
  KindIcon,
  type KindIconRegistry,
} from "@compozy/ui";

import {
  LOOP_CALL_TOOL_ICON,
  LOOP_NODE_KIND_ICONS,
  loopNodeClassIcon,
} from "../../lib/loop-node-kind-icons";
import {
  filterPaletteItems,
  loopPaletteItems,
  paletteKindKey,
  type PaletteItem,
} from "../../lib/loop-palette";
import { MonoTag } from "../mono-tag";

const QUICK_ADD_KIND_ICONS = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

const OPEN_KIND_GLYPH = loopNodeClassIcon({ nodeClass: "action" });

export interface LoopEditorQuickAddNodeOption {
  id: string;
  kind: string;
  label: string;
}

export interface LoopEditorQuickAddProps {
  open: boolean;

  nodes: readonly LoopEditorQuickAddNodeOption[];

  readOnly?: boolean;
  onOpenChange: (open: boolean) => void;
  onAddNode: (item: PaletteItem) => void;
  onRevealNode: (nodeId: string) => void;
}

type QuickAddPanelProps = Omit<LoopEditorQuickAddProps, "open">;

function paletteGlyph(item: PaletteItem) {
  return loopNodeClassIcon({
    nodeClass: item.nodeClass,
    isFanOut: item.kindLabel === "fan-out",
    isGate: item.kindLabel === "gate",
  });
}

function matchesNode(option: LoopEditorQuickAddNodeOption, needle: string): boolean {
  if (needle === "") return true;
  return `${option.id} ${option.kind} ${option.label}`.toLowerCase().includes(needle);
}

function QuickAddPanel({
  nodes,
  readOnly = false,
  onOpenChange,
  onAddNode,
  onRevealNode,
}: QuickAddPanelProps) {
  const [query, setQuery] = useState("");
  const needle = query.trim().toLowerCase();
  const items = readOnly ? [] : filterPaletteItems(loopPaletteItems(), query);
  const matchedNodes = nodes.filter(option => matchesNode(option, needle));

  const handleKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Escape") return;

    event.preventDefault();
    onOpenChange(false);
  };

  return (
    <Command data-testid="loop-editor-quick-add" onKeyDown={handleKeyDown} shouldFilter={false}>
      <CommandInput
        autoFocus
        data-testid="loop-quick-add-input"
        onValueChange={setQuery}
        placeholder={readOnly ? "Jump to a node…" : "Add a node or jump to one…"}
        value={query}
      />
      <CommandList className="max-h-[46vh]">
        <CommandEmpty data-testid="loop-quick-add-empty">
          {readOnly ? `No nodes match “${needle}”.` : `No kinds or nodes match “${needle}”.`}
        </CommandEmpty>
        {items.length > 0 ? (
          <CommandGroup heading="Add node">
            {items.map(item => (
              <CommandItem
                data-testid={`loop-quick-add-item-${item.kindLabel}`}
                key={item.kindLabel}
                onSelect={() => {
                  onAddNode(item);
                  onOpenChange(false);
                }}
                value={item.label}
              >
                <KindIcon
                  className="size-3.5"
                  fallback={paletteGlyph(item)}
                  kind={paletteKindKey(item.kindLabel)}
                  registry={QUICK_ADD_KIND_ICONS}
                  size="xs"
                  tone="muted"
                />
                <span className="min-w-0 truncate">{item.label}</span>
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
        {matchedNodes.length > 0 ? (
          <CommandGroup heading="Jump to node">
            {matchedNodes.map(option => (
              <CommandItem
                data-testid={`loop-quick-add-node-${option.id}`}
                key={option.id}
                onSelect={() => {
                  onRevealNode(option.id);
                  onOpenChange(false);
                }}
                value={option.id}
              >
                <KindIcon
                  className="size-3.5"
                  fallback={OPEN_KIND_GLYPH}
                  kind={option.kind}
                  registry={QUICK_ADD_KIND_ICONS}
                  size="xs"
                  tone="muted"
                />
                <span className="min-w-0 truncate">{option.label}</span>
                {option.kind === "" ? null : (
                  <MonoTag className="ml-auto shrink-0 text-pill-group-badge tracking-[0.07em] text-faint">
                    {option.kind}
                  </MonoTag>
                )}
              </CommandItem>
            ))}
          </CommandGroup>
        ) : null}
      </CommandList>
      <div className="flex items-center gap-5 border-t border-line px-2.5 py-2 text-micro text-subtle">
        <span className="flex items-center gap-1.5">
          <Kbd>↑↓</Kbd>move
        </span>
        <span className="flex items-center gap-1.5">
          <Kbd>⏎</Kbd>select
        </span>
        <span className="ml-auto flex items-center gap-1.5">
          <Kbd>esc</Kbd>cancel
        </span>
      </div>
    </Command>
  );
}

export function LoopEditorQuickAdd({
  open,
  nodes,
  readOnly = false,
  onOpenChange,
  onAddNode,
  onRevealNode,
}: LoopEditorQuickAddProps) {
  return (
    <CommandDialog
      description={
        readOnly
          ? "Jump to a node already on the canvas"
          : "Add a node to the canvas or jump to one already on it"
      }
      onOpenChange={onOpenChange}
      open={open}
      title={readOnly ? "Jump to node" : "Quick add"}
    >
      <QuickAddPanel
        nodes={nodes}
        onAddNode={onAddNode}
        onOpenChange={onOpenChange}
        onRevealNode={onRevealNode}
        readOnly={readOnly}
      />
    </CommandDialog>
  );
}
