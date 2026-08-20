import type { RefObject } from "react";

import { Command, CommandEmpty, CommandInput, CommandList } from "@compozy/ui";

import type { OsPaletteRootModel } from "../hooks/use-os-palette-root";
import { acceptGhostCompletion } from "../lib/ranking/ghost";
import { OsPaletteDomainSections } from "./os-palette-domain-sections";
import { OsPaletteEntitySections } from "./os-palette-entity-sections";
import { OsPaletteFooter } from "./os-palette-footer";
import { OsPaletteResults } from "./os-palette-results";

const EMPTY_COPY = "No matches — try an app, a session title, or an action.";
const DESTINATION_EMPTY_COPY = "No matches — try an app or a session title.";
const ZERO_ELIGIBLE_COPY = "Nothing can open in this tab yet.";

export interface OsPaletteRootFrameProps {
  model: OsPaletteRootModel;
  /** The highlighted row value, resolved against the live list. */
  selected: string;
  values: readonly string[];
  contentRef: RefObject<HTMLDivElement | null>;
  /** Command ids the daemon is currently running for this client. */
  pending: ReadonlySet<string>;
  onSelectionChange: (value: string) => void;
}

/**
 * The palette at rest: query, results, footer.
 *
 * cmdk filters nothing (`shouldFilter={false}`): the projection decides
 * membership and order, and `resolveCommandSelection` keeps the highlight steady
 * while a live catalog churns underneath it — which is also what makes the
 * action panel's nearest-neighbour fallback work when its row disappears.
 */
export function OsPaletteRootFrame({
  model,
  selected,
  values,
  contentRef,
  pending,
  onSelectionChange,
}: OsPaletteRootFrameProps) {
  const domainBusy = model.domainSections.some(
    section => section.loading || section.error !== null
  );
  const empty = values.length === 0 && !domainBusy;
  // The actions hint is only true while there is a row to act on, and its chord
  // comes from the keymap the daemon serves.
  const paletteToggleChords = model.registry.byId.get("palette.open")?.chords ?? [];
  const actionsChord =
    empty ||
    model.destination ||
    selected === model.fallback?.value ||
    paletteToggleChords.length === 0
      ? undefined
      : paletteToggleChords.join(" / ");
  return (
    <Command
      data-destination={model.destination ? "" : undefined}
      data-stale={model.registry.stale ? "" : undefined}
      data-testid="os-command-palette"
      ref={contentRef}
      shouldFilter={false}
      value={selected}
      onValueChange={onSelectionChange}
    >
      <div className="relative">
        <CommandInput
          autoFocus
          placeholder={
            model.destination ? "Open in this tab…" : "Search apps, sessions, and actions…"
          }
          value={model.query}
          onKeyDown={event => {
            if (event.key !== "ArrowRight") return;
            const accepted = acceptGhostCompletion(
              model.query,
              model.ghostTail,
              event.currentTarget.selectionStart,
              event.currentTarget.selectionEnd
            );
            if (accepted === null) return;
            event.preventDefault();
            model.setQuery(accepted);
          }}
          onValueChange={model.setQuery}
        />
        {model.ghostTail === null ? null : (
          <div
            aria-hidden="true"
            className="pointer-events-none absolute top-1 left-11 flex h-control-compact items-center text-small-body"
            data-testid="os-palette-ghost"
          >
            <span className="invisible whitespace-pre">{model.query}</span>
            <span className="whitespace-pre text-faint">{model.ghostTail}</span>
          </div>
        )}
      </div>
      <CommandList className="max-h-[46vh] px-2 pb-3">
        {empty ? (
          <CommandEmpty data-testid="os-palette-empty">
            {model.destinationEmpty
              ? ZERO_ELIGIBLE_COPY
              : model.destination
                ? DESTINATION_EMPTY_COPY
                : EMPTY_COPY}
          </CommandEmpty>
        ) : null}
        <OsPaletteResults
          fallback={model.fallback}
          fallbackPending={model.fallbackPending}
          pending={pending}
          sections={model.sections}
          onSelect={model.runCommand}
          onSelectFallback={model.runFallback}
        />
        <OsPaletteEntitySections
          destination={model.destination}
          entities={model.entities}
          onGoToTab={model.goToTab}
          onOpenSession={model.openSession}
          onSelectWorktree={model.selectWorktree}
        />
        <OsPaletteDomainSections onOpen={model.openDomainRow} sections={model.domainSections} />
      </CommandList>
      <OsPaletteFooter
        {...(actionsChord === undefined ? {} : { actionsChord })}
        enterHint={model.destination ? "open here" : "open"}
      />
    </Command>
  );
}
