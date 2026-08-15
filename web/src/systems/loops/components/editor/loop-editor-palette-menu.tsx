import { Plus } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  KindIcon,
  type KindIconRegistry,
} from "@compozy/ui";

import {
  LOOP_CALL_TOOL_ICON,
  LOOP_NODE_KIND_ICONS,
  loopNodeClassIcon,
} from "../../lib/loop-node-kind-icons";
import { LOOP_PALETTE, type PaletteItem } from "../../lib/loop-palette";

interface LoopEditorPaletteMenuProps {
  onAddNode: (item: PaletteItem) => void;
  disabled?: boolean;
}

const LOOP_EDITOR_KIND_ICON_REGISTRY = {
  ...LOOP_NODE_KIND_ICONS,
  "": LOOP_CALL_TOOL_ICON,
} satisfies KindIconRegistry;

function paletteKindKey(kindLabel: string): string {
  return kindLabel === "tool…" ? "" : kindLabel;
}

/**
 * Compact Add-node menu for viewports below `lg`, where the side palette is
 * `hidden`. Groups and items mirror `LOOP_PALETTE` and call the same `onAddNode`.
 */
export function LoopEditorPaletteMenu({ onAddNode, disabled = false }: LoopEditorPaletteMenuProps) {
  return (
    <div className="lg:hidden" data-testid="loop-editor-palette-menu">
      <DropdownMenu>
        <DropdownMenuTrigger
          render={
            <Button
              data-testid="loop-editor-add-node"
              disabled={disabled}
              size="sm"
              type="button"
              variant="outline"
            />
          }
        >
          <Plus aria-hidden="true" className="size-3.5" />
          Add node
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="min-w-52">
          {LOOP_PALETTE.map((group, index) => (
            <DropdownMenuGroup key={group.label}>
              {index > 0 ? <DropdownMenuSeparator /> : null}
              <DropdownMenuLabel>{group.label}</DropdownMenuLabel>
              {group.items.map(item => (
                <DropdownMenuItem
                  data-testid={`loop-palette-menu-item-${item.kindLabel}`}
                  disabled={disabled}
                  key={item.label}
                  onClick={() => onAddNode(item)}
                >
                  <KindIcon
                    className="size-3.5"
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
                  {item.label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuGroup>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
}
