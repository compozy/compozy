import { CommandGroup, cn } from "@compozy/ui";

import {
  overflowNote,
  type PaletteAgentFallback,
  type PaletteSection,
} from "../lib/cmd-palette-sections";
import type { ResolvedPaletteCommand } from "../lib/cmd-palette-types";
import { OsPaletteCommandRow } from "./os-palette-command-row";
import { OsPaletteFallbackRow } from "./os-palette-fallback-row";

export interface OsPaletteResultsProps {
  sections: readonly PaletteSection[];
  /** Command ids the daemon is currently running for this client. */
  pending: ReadonlySet<string>;
  onSelect: (command: ResolvedPaletteCommand) => void;
  fallback: PaletteAgentFallback | null;
  fallbackPending: boolean;
  onSelectFallback: (query: string) => void;
}

/**
 * The registry projection (`_uiux.md` S1/S10).
 *
 * Replaces the four hand-written group components: every row here comes from
 * the one client registry, so nothing renders that the daemon does not carry
 * and nothing the daemon carries is invisible because a component forgot it.
 * Groups after the first open with a full-bleed rule; a capped group states its
 * exact overflow rather than truncating in silence (BR-18).
 */
export function OsPaletteResults({
  sections,
  pending,
  onSelect,
  fallback,
  fallbackPending,
  onSelectFallback,
}: OsPaletteResultsProps) {
  return (
    <>
      {sections.map((section, index) => {
        const note = overflowNote(section);
        return (
          <CommandGroup
            className={cn(
              "px-2 pb-0.5",
              index > 0 && "mt-2 border-t border-line-soft pt-2",
              "**:[[cmdk-group-heading]]:text-faint"
            )}
            data-testid={`os-palette-section-${section.title.toLowerCase()}`}
            heading={section.title}
            key={section.title}
          >
            {section.commands.map(command => (
              <OsPaletteCommandRow
                command={command}
                key={command.id}
                pending={pending.has(command.id)}
                onSelect={onSelect}
              />
            ))}
            {note === null ? null : (
              <p
                className="px-3 pt-1 pb-0.5 text-small-body text-faint"
                data-testid={`os-palette-overflow-${section.title.toLowerCase()}`}
              >
                {note}
              </p>
            )}
          </CommandGroup>
        );
      })}
      {fallback === null ? null : (
        <OsPaletteFallbackRow
          fallback={fallback}
          pending={fallbackPending}
          onSelect={onSelectFallback}
        />
      )}
    </>
  );
}
