import {
  Button,
  Kbd,
  ToggleGroup,
  ToggleGroupItem,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { FileSearch } from "@compozy/ui/icons";

import type { ExplorerPane } from "../hooks/use-files-explorer";

/**
 * The explorer's left-pane switch (`_uiux.md` S12): Files ↔ Search, plus the Go to file
 * button (US-013.AC-1).
 */
export function ExplorerPaneSwitch({
  pane,
  onPaneChange,
  onGoToFile,
  primaryKey,
}: {
  pane: ExplorerPane;
  onPaneChange: (next: ExplorerPane) => void;
  onGoToFile: () => void;
  /** "⌘" or "Ctrl", for the shortcut hints. */
  primaryKey: string;
}) {
  return (
    <div className="flex shrink-0 items-center gap-1 px-2 pt-2 pb-1.5">
      <ToggleGroup
        size="sm"
        aria-label="Explorer view"
        value={[pane]}
        onValueChange={value => {
          const next = value[0];
          if (next === "files" || next === "search") onPaneChange(next);
        }}
        data-testid="files-pane-switch"
      >
        <ToggleGroupItem value="files" data-testid="files-pane-files">
          Files
        </ToggleGroupItem>
        <ToggleGroupItem value="search" data-testid="files-pane-search">
          Search
        </ToggleGroupItem>
      </ToggleGroup>
      <div className="flex-1" />
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              type="button"
              variant="quiet"
              size="icon-xs"
              aria-label="Go to file"
              data-testid="files-go-to-file"
              onClick={onGoToFile}
            />
          }
        >
          <FileSearch aria-hidden className="size-3.5" />
        </TooltipTrigger>
        <TooltipContent>
          Go to file <Kbd>{primaryKey}P</Kbd>
        </TooltipContent>
      </Tooltip>
    </div>
  );
}
