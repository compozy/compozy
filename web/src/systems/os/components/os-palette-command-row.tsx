import { CommandItem, CommandShortcut, KindIcon, Pill, StatusDot, cn } from "@compozy/ui";

import {
  CMD_PALETTE_ICON_FALLBACK,
  cmdPaletteIconRegistry,
  isEmojiIcon,
} from "../lib/cmd-palette-icons";
import type { ResolvedPaletteCommand } from "../lib/cmd-palette-types";

export interface OsPaletteCommandRowProps {
  command: ResolvedPaletteCommand;
  /** True while the daemon is running this command for this client (US-017.AC-2). */
  pending?: boolean;
  onSelect: (command: ResolvedPaletteCommand) => void;
}

function extensionName(source: string): string | null {
  return source.startsWith("ext.") ? source.slice("ext.".length) : null;
}

/**
 * One command row.
 *
 * A disabled row keeps its full-contrast label and carries the runtime's reason
 * verbatim in its own hint slot — never folded into the label as prose (BR-8,
 * US-037.AC-1). The standard 50% disabled dimming is overridden here because it
 * drags that reason below the AA floor; the row reads as disabled through
 * colour instead (authorized delta, `DESIGN-NOTES.md`).
 *
 * It stays *selectable*, and only `aria-disabled` says otherwise. cmdk skips a
 * `disabled` item during keyboard navigation, which would put the row's reason
 * and its meta-actions out of reach of the keyboard entirely (US-014.EC-2). The
 * seam is what refuses the run, reporting the same reason the row shows.
 */
export function OsPaletteCommandRow({ command, pending, onSelect }: OsPaletteCommandRowProps) {
  const extension = extensionName(command.source);
  const emoji = isEmojiIcon(command.icon);
  return (
    <CommandItem
      forceMount
      aria-busy={pending === true ? true : undefined}
      aria-disabled={command.available ? undefined : true}
      className={cn("h-10 gap-3 px-3", !command.available && "text-muted")}
      data-palette-row={command.id}
      data-testid={`os-palette-command-${command.id}`}
      key={command.id}
      value={command.id}
      onSelect={() => onSelect(command)}
    >
      <span
        aria-hidden="true"
        className={cn(
          "flex size-[18px] shrink-0 items-center justify-center rounded-full bg-canvas-tint",
          command.available ? "text-subtle" : "text-faint"
        )}
      >
        {emoji ? (
          <span className="text-badge leading-none">{command.icon}</span>
        ) : (
          <KindIcon
            className="size-3"
            fallback={CMD_PALETTE_ICON_FALLBACK}
            kind={command.icon}
            registry={cmdPaletteIconRegistry}
          />
        )}
      </span>
      <span className="min-w-0 truncate">
        {command.title}
        {command.alias ? <span className="text-muted"> ({command.alias})</span> : null}
      </span>
      {extension ? (
        <Pill className="shrink-0 font-mono" size="xs" tone="info">
          {extension}
        </Pill>
      ) : null}
      {command.available ? null : (
        <span
          className="min-w-0 truncate text-small-body text-subtle"
          data-slot="os-palette-reason"
        >
          {command.reason}
        </span>
      )}
      {pending === true ? (
        // Indeterminate by construction: the runtime reports that it is running,
        // not how far along it is, so neither does this (SD-007).
        <span
          className="ms-auto flex shrink-0 items-center gap-1.5 text-small-body text-subtle"
          data-testid={`os-palette-pending-${command.id}`}
        >
          <StatusDot className="motion-safe:animate-pulse" label="Running" tone="accent" />
          pending
        </span>
      ) : command.chords.length > 0 ? (
        <CommandShortcut>{command.chords.join(" / ")}</CommandShortcut>
      ) : null}
    </CommandItem>
  );
}
