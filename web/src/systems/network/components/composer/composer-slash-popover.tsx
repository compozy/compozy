import { useEffect, useEffectEvent, useRef } from "react";

import { Command, CommandEmpty, CommandItem, CommandList, CommandShortcut } from "@agh/ui";

import { cn } from "@/lib/utils";
import { filterSlashCommandEntries, type SlashCommandEntry } from "./composer-slash-popover.logic";

export type { SlashCommandEntry } from "./composer-slash-popover.logic";

export interface ComposerSlashPopoverProps {
  open: boolean;
  filterValue: string;
  onSelect: (entry: SlashCommandEntry) => void;
  onClose: () => void;
  className?: string;
}

export function ComposerSlashPopover({
  open,
  filterValue,
  onSelect,
  onClose,
  className,
}: ComposerSlashPopoverProps) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const closePopover = useEffectEvent(onClose);

  useEffect(() => {
    if (!open) {
      return undefined;
    }
    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape") {
        event.preventDefault();
        closePopover();
      }
    }
    window.addEventListener("keydown", handleKey);
    return () => window.removeEventListener("keydown", handleKey);
  }, [open]);

  if (!open) {
    return null;
  }

  const entries = filterSlashCommandEntries(filterValue);

  return (
    <Command
      aria-label="Slash commands"
      className={cn(
        "absolute bottom-full left-0 mb-2 w-72 rounded-mono-badge border border-line bg-canvas p-1 text-small-body",
        className
      )}
      data-testid="network-composer-slash-popover"
      ref={containerRef}
    >
      <CommandList>
        {entries.length === 0 ? <CommandEmpty>No matching commands.</CommandEmpty> : null}
        {entries.map(entry => (
          <CommandItem
            aria-disabled={entry.disabled ? "true" : "false"}
            className={cn(
              "items-baseline rounded-chip px-3 py-2",
              entry.disabled ? "cursor-not-allowed text-subtle" : null
            )}
            data-disabled={entry.disabled ? "true" : "false"}
            data-testid={`network-composer-slash-option-${entry.command}`}
            disabled={entry.disabled}
            key={entry.command}
            onSelect={() => {
              if (!entry.disabled) {
                onSelect(entry);
              }
            }}
            title={entry.disabled ? entry.disabledReason : undefined}
            value={entry.command}
          >
            <span
              className={cn(
                "font-mono text-xs tracking-mono",
                entry.disabled ? "text-subtle" : "text-fg"
              )}
            >
              /{entry.command}
            </span>
            <span className="truncate text-xs text-subtle">{entry.description}</span>
            {entry.disabled ? <CommandShortcut>Post-MVP</CommandShortcut> : null}
          </CommandItem>
        ))}
      </CommandList>
    </Command>
  );
}
