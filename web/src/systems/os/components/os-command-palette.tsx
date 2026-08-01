import { Command, CommandDialog, CommandEmpty, CommandInput, CommandList } from "@compozy/ui";

import { useOsCommandPalette } from "../hooks/use-os-command-palette";
import { OsCommandPaletteResults } from "./os-command-palette-results";
import { OsCommandPaletteShellActions } from "./os-command-palette-shell-actions";
import { OsCommandPaletteWindowActions } from "./os-command-palette-window-actions";

export interface OsCommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Opens the daemon-backed Desktops overview (⇧⌘S parity). */
  onOpenDesktops?: () => void;
  /** Toggles the global Sessions catalog modal. */
  onToggleSessions?: () => void;
}

/**
 * The global ⌘K palette (ADR-005): a desktop-level overlay above the win-layer
 * that delegates each independently-owned result group to a focused component.
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
  const destination = model.destinationWindowId !== null;

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={destination ? "Choose a destination" : "Command palette"}
      description={
        destination ? "Pick the surface this tab becomes" : "Search apps, sessions, and actions"
      }
      // Compact clamps to the phone canvas (os-v2.css `.palette-overlay{padding-top:9vh}`).
      className="top-[9vh] min-[960px]:top-[16vh] sm:max-w-(--width-modal-sm)"
    >
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
          <OsCommandPaletteResults destination={destination} model={model} />
          <OsCommandPaletteWindowActions destination={destination} model={model} />
          <OsCommandPaletteShellActions destination={destination} model={model} />
        </CommandList>
      </Command>
    </CommandDialog>
  );
}
