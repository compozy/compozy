import { useState } from "react";

import {
  Command,
  CommandDialog,
  CommandEmpty,
  CommandInput,
  CommandList,
  resolveCommandSelection,
} from "@compozy/ui";

import { useCmdPaletteDispatch } from "../hooks/use-cmd-palette-dispatch";
import { useOsPaletteRoot } from "../hooks/use-os-palette-root";
import { useOsPaletteViewStack } from "../hooks/use-os-palette-view-stack";
import { paletteViewDefinition } from "../lib/palette-view-registry";
import { acceptGhostCompletion } from "../lib/ranking/ghost";
import { OsPaletteEntitySections } from "./os-palette-entity-sections";
import { OsPaletteDomainSections } from "./os-palette-domain-sections";
import { OsPaletteFooter } from "./os-palette-footer";
import { OsPaletteResults } from "./os-palette-results";
import { OsPaletteViewStack } from "./os-palette-view-stack";

export interface OsCommandPaletteProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** The bound dispatch seam; the shell owns the coordinators it closes over. */
  dispatch: ReturnType<typeof useCmdPaletteDispatch>;
}

const EMPTY_COPY = "No matches — try an app, a session title, or an action.";
const DESTINATION_EMPTY_COPY = "No matches — try an app or a session title.";
const ZERO_ELIGIBLE_COPY = "Nothing can open in this tab yet.";

/**
 * The global ⌘K palette: a desktop-level overlay above the win-layer.
 *
 * Every row is a projection of the one client registry (ADR-001), so this
 * component owns presentation and nothing else — no command lists, no
 * availability rules, no dispatch branches. cmdk filters nothing
 * (`shouldFilter={false}`): the projection decides membership and order, and
 * `resolveCommandSelection` keeps the highlight steady while a live catalog
 * churns underneath it.
 *
 * The root is one level of a stack. A pushed view replaces the root's results
 * with its own — a scoped picker inside the surface the operator already
 * reaches for, rather than another window to learn.
 */
export function OsCommandPalette({ open, onOpenChange, dispatch }: OsCommandPaletteProps) {
  const viewStack = useOsPaletteViewStack();
  const model = useOsPaletteRoot({
    open,
    onOpenChange,
    dispatch: (command, query) => void dispatch.run(command, undefined, query),
  });
  const activeView =
    viewStack.activeViewId === null ? null : paletteViewDefinition(viewStack.activeViewId);
  const values = [
    ...model.sections.flatMap(section => section.commands.map(command => command.id)),
    ...model.entities.sessions.map(session => `session:${session.sessionId}`),
    ...(model.destination ? [] : model.entities.tabs.map(tab => `tab:${tab.windowId}`)),
    ...(model.destination ? [] : model.entities.worktrees.map(entry => `worktree:${entry.key}`)),
    ...model.domainSections.flatMap(section => section.rows.map(row => row.key)),
  ];
  const [selection, setSelection] = useState<{ previous: readonly string[]; value: string }>(
    () => ({ previous: values, value: values[0] ?? "" })
  );
  const selected = resolveCommandSelection(selection.previous, values, selection.value);
  const empty = values.length === 0;

  return (
    <CommandDialog
      open={open}
      onOpenChange={onOpenChange}
      title={activeView?.title ?? (model.destination ? "Choose a destination" : "Command palette")}
      description={
        activeView?.description ??
        (model.destination
          ? "Pick the surface this tab becomes"
          : "Search apps, sessions, and actions")
      }
      // Compact clamps to the phone canvas (os-v2.css `.palette-overlay{padding-top:9vh}`).
      className="top-[9vh] min-[960px]:top-[16vh] sm:max-w-(--width-modal-sm)"
    >
      {viewStack.activeViewId === null ? (
        <Command
          data-testid="os-command-palette"
          data-destination={model.destination ? "" : undefined}
          data-stale={model.registry.stale ? "" : undefined}
          shouldFilter={false}
          value={selected}
          onValueChange={next => setSelection({ previous: values, value: next })}
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
            <OsPaletteResults onSelect={model.runCommand} sections={model.sections} />
            <OsPaletteEntitySections
              destination={model.destination}
              entities={model.entities}
              onGoToTab={model.goToTab}
              onOpenSession={model.openSession}
              onSelectWorktree={model.selectWorktree}
            />
            <OsPaletteDomainSections sections={model.domainSections} onOpen={model.openDomainRow} />
          </CommandList>
          <OsPaletteFooter enterHint={model.destination ? "open here" : "open"} />
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
