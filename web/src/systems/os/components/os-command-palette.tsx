import { Command, CommandDialog, CommandEmpty, CommandInput, CommandList } from "@compozy/ui";

import { useOsCommandPalette } from "../hooks/use-os-command-palette";
import { useOsPaletteViewStack } from "../hooks/use-os-palette-view-stack";
import { paletteViewDefinition } from "../lib/palette-view-registry";
import { OsCommandPaletteResults } from "./os-command-palette-results";
import { OsCommandPaletteShellActions } from "./os-command-palette-shell-actions";
import { OsCommandPaletteViews } from "./os-command-palette-views";
import { OsCommandPaletteWindowActions } from "./os-command-palette-window-actions";
import { OsPaletteFooter } from "./os-palette-footer";
import { OsPaletteViewStack } from "./os-palette-view-stack";

export interface OsCommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Opens the daemon-backed Desktops overview. */
  onOpenDesktops?: () => void;
  /** Toggles the global Sessions catalog modal. */
  onToggleSessions?: () => void;
}

/**
 * The global ⌘K palette (ADR-005): a desktop-level overlay above the win-layer
 * that delegates each independently-owned result group to a focused component.
 *
 * The root is one level of a stack (ADR-003). A pushed view replaces the root's
 * results with its own — a scoped picker inside the surface the operator
 * already reaches for, rather than another window to learn.
 */
export function OsCommandPalette({
  open,
  onOpenChange,
  onOpenDesktops,
  onToggleSessions,
}: OsCommandPaletteProps) {
  const model = useOsCommandPalette(open, onOpenChange, {
    onOpenDesktops,
    onToggleSessions,
  });
  const viewStack = useOsPaletteViewStack();
  const destination = model.destinationWindowId !== null;
  const activeView =
    viewStack.activeViewId === null ? null : paletteViewDefinition(viewStack.activeViewId);

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={activeView?.title ?? (destination ? "Choose a destination" : "Command palette")}
      description={
        activeView?.description ??
        (destination ? "Pick the surface this tab becomes" : "Search apps, sessions, and actions")
      }
      // Compact clamps to the phone canvas (os-v2.css `.palette-overlay{padding-top:9vh}`).
      className="top-[9vh] min-[960px]:top-[16vh] sm:max-w-(--width-modal-sm)"
    >
      {viewStack.activeViewId === null ? (
        <Command
          data-testid="os-command-palette"
          data-destination={destination ? "" : undefined}
          shouldFilter
        >
          <CommandInput
            autoFocus
            placeholder={destination ? "Open in this tab…" : "Search apps, sessions, and actions…"}
          />
          <CommandList className="max-h-[46vh]">
            <CommandEmpty>
              {destination
                ? "No matches — try an app or a session title."
                : "No matches — try an app, a session title, or an action."}
            </CommandEmpty>
            {destination ? null : <OsCommandPaletteViews onPushView={viewStack.push} />}
            <OsCommandPaletteResults destination={destination} model={model} />
            <OsCommandPaletteWindowActions destination={destination} model={model} />
            <OsCommandPaletteShellActions destination={destination} model={model} />
          </CommandList>
          <OsPaletteFooter enterHint={destination ? "open here" : "open"} />
        </Command>
      ) : (
        <OsPaletteViewStack
          // Each level is its own instance: pushing or popping starts the new
          // level's search and selection clean instead of inheriting them.
          key={`${viewStack.stack.length}:${viewStack.activeViewId}`}
          viewId={viewStack.activeViewId}
          breadcrumb={viewStack.breadcrumb}
          onPop={viewStack.pop}
          onDismiss={() => onOpenChange(false)}
        />
      )}
    </CommandDialog>
  );
}
