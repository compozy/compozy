import { useState } from "react";
import { GitBranch, Plus, TerminalSquare } from "@compozy/ui/icons";

import {
  cn,
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Kbd,
} from "@compozy/ui";
import { terminalDisplayTitle, type TerminalInfo } from "@/systems/terminal";
import { WorkspaceBadge, workspaceBadgeIdentity } from "@/systems/workspace";

import type { TerminalLauncherDestination } from "../lib/terminal-launcher-model";

export interface TerminalLauncherProps {
  destinations: readonly TerminalLauncherDestination[];
  /** The destination selected on open; Enter opens a terminal there. */
  defaultDestination: string;
  /** Live terminals to re-attach. Exited ones never reach the launcher. */
  running: readonly TerminalInfo[];
  /** Project id → name, for each running terminal's badge. */
  projectNames: ReadonlyMap<string, string>;
  /** All profiles is read-only: nothing new starts, running ones still open. */
  readOnly?: boolean;
  /** A new terminal is starting; rows wait for it instead of starting a second one. */
  busy?: boolean;
  /** The running list could not load; new terminals still open. */
  runningError?: string | null;
  onOpenNew: (destination: TerminalLauncherDestination) => void;
  onAttach: (terminal: TerminalInfo) => void;
  className?: string;
}

const NEW_PREFIX = "new:";
const ATTACH_PREFIX = "attach:";

/**
 * The Terminal entry point: pick where a new terminal opens, or go back to a
 * running one. Keyboard-first — the default destination is selected, so one
 * Enter opens it; typing filters both groups.
 */
export function TerminalLauncher({
  destinations,
  defaultDestination,
  running,
  projectNames,
  readOnly = false,
  busy = false,
  runningError = null,
  onOpenNew,
  onAttach,
  className,
}: TerminalLauncherProps) {
  const defaultValue =
    !readOnly && destinations.some(destination => destination.value === defaultDestination)
      ? `${NEW_PREFIX}${defaultDestination}`
      : running[0]
        ? `${ATTACH_PREFIX}${running[0].id}`
        : undefined;
  // `null` follows the default, so a default that settles later (the catalog
  // arriving) re-selects; a row the operator moves to, or a search, sticks.
  const [chosen, setChosen] = useState<string | null>(null);
  return (
    <Command
      className={cn("h-auto", className)}
      data-testid="terminal-launcher"
      onValueChange={value => setChosen(value === defaultValue ? null : value)}
      value={chosen ?? defaultValue ?? ""}
    >
      <CommandInput autoFocus placeholder="Open a terminal in…" variant="quiet" />
      <CommandList className="max-h-[min(24rem,50vh)]">
        <CommandEmpty>No project, worktree, or terminal matches.</CommandEmpty>
        {readOnly ? null : (
          <CommandGroup heading="New terminal">
            {destinations.map(destination => (
              <CommandItem
                data-testid={`terminal-launcher-new-${destination.value}`}
                disabled={busy || destination.unavailableReason !== null}
                key={destination.value}
                keywords={
                  destination.projectLabel
                    ? [destination.label, destination.projectLabel]
                    : [destination.label]
                }
                onSelect={() => onOpenNew(destination)}
                value={`${NEW_PREFIX}${destination.value}`}
              >
                {destination.worktreeId ? (
                  <GitBranch aria-hidden="true" className="ms-4 text-subtle" />
                ) : (
                  <Plus aria-hidden="true" className="text-subtle" />
                )}
                <span className="min-w-0 truncate">{destination.label}</span>
                {destination.projectLabel ? (
                  <span className="ms-auto min-w-0 truncate ps-3 text-micro text-subtle">
                    {destination.projectLabel}
                  </span>
                ) : null}
                {destination.unavailableReason ? (
                  <span className="ms-auto ps-3 text-micro text-subtle">
                    {destination.unavailableReason}
                  </span>
                ) : null}
              </CommandItem>
            ))}
          </CommandGroup>
        )}
        {running.length > 0 ? (
          <CommandGroup heading="Running">
            {running.map(terminal => {
              const project = workspaceBadgeIdentity(terminal.workspace_id, projectNames);
              return (
                <CommandItem
                  data-testid={`terminal-launcher-running-${terminal.id}`}
                  key={terminal.id}
                  keywords={[
                    terminalDisplayTitle(terminal),
                    project.name,
                    ...(readOnly ? [terminal.profile_name] : []),
                  ]}
                  onSelect={() => onAttach(terminal)}
                  value={`${ATTACH_PREFIX}${terminal.id}`}
                >
                  <TerminalSquare aria-hidden="true" className="text-subtle" />
                  <span className="min-w-0 truncate">{terminalDisplayTitle(terminal)}</span>
                  <span className="ms-auto flex min-w-0 items-center gap-2 ps-3">
                    {/* All profiles: the owner tells same-titled terminals apart. */}
                    {readOnly ? (
                      <span className="text-micro text-subtle">{terminal.profile_name}</span>
                    ) : null}
                    <WorkspaceBadge identity={project} />
                  </span>
                </CommandItem>
              );
            })}
          </CommandGroup>
        ) : null}
        {runningError ? (
          <p className="px-3 py-2 text-small-body text-muted-foreground" role="alert">
            {runningError}
          </p>
        ) : null}
      </CommandList>
      <div className="flex items-center gap-5 border-t border-line px-2.5 py-2 text-micro text-subtle">
        <span className="flex items-center gap-1.5">
          <Kbd>↑↓</Kbd>move
        </span>
        <span className="flex items-center gap-1.5">
          <Kbd>⏎</Kbd>open
        </span>
      </div>
    </Command>
  );
}
