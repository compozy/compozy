import { CommandItem, CommandShortcut, KindIcon, Pill, StatusDot, cn } from "@compozy/ui";
import { useId } from "react";

import {
  CMD_PALETTE_ICON_FALLBACK,
  cmdPaletteIconRegistry,
  isEmojiIcon,
} from "../lib/cmd-palette-icons";
import type { ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import { paletteRowClass } from "../lib/palette-view-inset";
import { parseExtensionName } from "../lib/palette-view-registry";

export interface OsPaletteCommandRowProps {
  command: ResolvedPaletteCommand;
  /** True while the daemon is running this command for this client (US-017.AC-2). */
  pending?: boolean;
  onSelect: (command: ResolvedPaletteCommand) => void;
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
 * The row remains an enabled option because selecting it is still meaningful:
 * keyboard selection exposes its reason and its meta-actions (US-014.EC-2).
 * Marking the option `aria-disabled` would falsely say that none of those
 * interactions are available and would make cmdk skip it during arrow-key
 * navigation. Instead, the visible reason is its accessible description and
 * the dispatch seam refuses the unavailable primary action with that reason.
 */
export function OsPaletteCommandRow({ command, pending, onSelect }: OsPaletteCommandRowProps) {
  const extension = parseExtensionName(command.source);
  const emoji = isEmojiIcon(command.icon);
  const reasonId = useId();

  return (
    <CommandItem
      forceMount
      aria-busy={pending === true ? true : undefined}
      aria-describedby={command.available ? undefined : reasonId}
      className={cn(paletteRowClass, !command.available && "text-muted")}
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
      <span className="min-w-0 truncate leading-none">
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
          aria-hidden="true"
          id={reasonId}
          className="min-w-0 truncate text-small-body leading-none text-subtle"
          data-slot="os-palette-reason"
        >
          {command.reason}
        </span>
      )}
      {pending === true ? (
        // Indeterminate by construction: the runtime reports that it is running,
        // not how far along it is, so neither does this (SD-007).
        <span
          className="ms-auto flex shrink-0 items-center gap-1.5 text-small-body leading-none text-subtle"
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
