import { CornerDownLeft } from "lucide-react";

import { CommandGroup, CommandItem, CommandShortcut, Icon } from "@compozy/ui";

import { PALETTE_VIEW_LIST, type PaletteViewId } from "../lib/palette-view-registry";

export interface OsCommandPaletteViewsProps {
  onPushView: (viewId: PaletteViewId) => void;
}

/**
 * The registered views, offered from the palette root.
 *
 * These rows are the one kind of result that does not close the palette: they
 * go a level deeper, so the trailing marker says "view" instead of leaving the
 * operator to find out by pressing ⏎.
 */
export function OsCommandPaletteViews({ onPushView }: OsCommandPaletteViewsProps) {
  return (
    <CommandGroup heading="Views">
      {PALETTE_VIEW_LIST.map(view => (
        <CommandItem
          key={view.id}
          value={`view ${view.title}`}
          data-testid={`os-palette-view-${view.id}`}
          onSelect={() => onPushView(view.id)}
        >
          <Icon as={view.icon} size="sm" className="text-muted" />
          {view.title}
          <CommandShortcut className="flex items-center gap-1">
            view
            <Icon as={CornerDownLeft} size="xs" />
          </CommandShortcut>
        </CommandItem>
      ))}
    </CommandGroup>
  );
}
