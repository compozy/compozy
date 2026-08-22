import { MenubarItem, MenubarShortcut } from "@compozy/ui";

import { usePaletteCommand } from "../../hooks/use-palette-registry";

export interface MenubarCommandItemProps {
  /** Registry command id; the menu curates order, the registry defines the item. */
  commandId: string;
  /** Runs the command through the one dispatch seam. */
  onRun: (commandId: string) => void;
}

/**
 * A menubar item projected from the registry (BR-17, `_uiux.md` S11).
 *
 * Menus curate grouping and order and nothing else: label, chord, availability
 * and reason all come from the same projection the palette renders, so the two
 * cannot drift and US-001.AC-4 holds by construction. There is deliberately no
 * label override — a menu-local string would be exactly the second source of
 * truth this task deletes. An item whose command becomes unavailable disables
 * in place with the runtime's reason; it never vanishes mid-session
 * (US-035.EC-1).
 */
export function MenubarCommandItem({ commandId, onRun }: MenubarCommandItemProps) {
  const command = usePaletteCommand(commandId);
  if (command === null) return null;
  return (
    <MenubarItem
      data-testid={`os-menubar-command-${commandId}`}
      disabled={!command.available}
      title={command.available ? undefined : command.reason}
      onClick={() => onRun(commandId)}
    >
      <span className="min-w-0 flex-1">
        <span className="block">{command.title}</span>
        {!command.available && command.reason ? (
          <span className="block text-micro text-muted">{command.reason}</span>
        ) : null}
      </span>
      {command.chords.length > 0 ? <MenubarShortcut>{command.chords[0]}</MenubarShortcut> : null}
    </MenubarItem>
  );
}
