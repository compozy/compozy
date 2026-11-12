import { useId } from "react";
import { ChevronRight, GitBranch, Plus, TerminalSquare } from "@compozy/ui/icons";

import {
  cn,
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  Kbd,
  TreeBranch,
  TreeBranchNode,
} from "@compozy/ui";
import { terminalDisplayTitle, type TerminalInfo } from "@/systems/terminal";
import { WorkspaceBadge, workspaceBadgeIdentity } from "@/systems/workspace";

import { useTerminalLauncherTree } from "../hooks/use-terminal-launcher-tree";
import {
  terminalLauncherAttachValue,
  terminalLauncherMatches,
  terminalLauncherNewValue,
  type TerminalLauncherDestination,
  type TerminalLauncherProjectView,
} from "../lib/terminal-launcher-model";

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

/**
 * The Terminal entry point: pick where a new terminal opens, or go back to a
 * running one. Keyboard-first — the default destination is selected, so one
 * Enter opens it; typing filters both groups. A project's worktrees fold under
 * it on the branched tree connector: → and ← unfold and fold the selected
 * project, and a search unfolds every project holding a matching worktree.
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
      ? terminalLauncherNewValue(defaultDestination)
      : running[0]
        ? terminalLauncherAttachValue(running[0].id)
        : undefined;
  const tree = useTerminalLauncherTree({ destinations, defaultValue, readOnly });
  const { projects, query } = tree;
  const branchIdPrefix = useId();
  const shownRunning = running.filter(terminal =>
    terminalLauncherMatches(query, [
      terminalDisplayTitle(terminal),
      workspaceBadgeIdentity(terminal.workspace_id, projectNames).name,
      ...(readOnly ? [terminal.profile_name] : []),
    ])
  );
  const hasWorktrees = !readOnly && projects.some(project => project.worktrees.length > 0);
  return (
    <Command
      className={cn("h-auto", className)}
      data-testid="terminal-launcher"
      onKeyDown={tree.onTreeKey}
      onValueChange={tree.select}
      shouldFilter={false}
      value={tree.selected}
    >
      <CommandInput
        autoFocus
        onValueChange={tree.search}
        placeholder="Open a terminal in…"
        value={query}
        variant="quiet"
      />
      <CommandList className="max-h-[min(24rem,50vh)]">
        <CommandEmpty>No project, worktree, or terminal matches.</CommandEmpty>
        {readOnly || projects.length === 0 ? null : (
          <CommandGroup heading="New terminal">
            {projects.map(project => (
              <TerminalLauncherProjectRows
                branchId={`${branchIdPrefix}-${project.destination.value}`}
                busy={busy}
                key={project.destination.value}
                onOpenNew={onOpenNew}
                onToggle={() => tree.toggle(project)}
                project={project}
              />
            ))}
          </CommandGroup>
        )}
        {shownRunning.length > 0 ? (
          <CommandGroup heading="Running">
            {shownRunning.map(terminal => {
              const project = workspaceBadgeIdentity(terminal.workspace_id, projectNames);
              return (
                <CommandItem
                  data-testid={`terminal-launcher-running-${terminal.id}`}
                  key={terminal.id}
                  onSelect={() => onAttach(terminal)}
                  value={terminalLauncherAttachValue(terminal.id)}
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
        {hasWorktrees ? (
          <span className="flex items-center gap-1.5">
            <Kbd>←→</Kbd>worktrees
          </span>
        ) : null}
        <span className="flex items-center gap-1.5">
          <Kbd>⏎</Kbd>open
        </span>
      </div>
    </Command>
  );
}

/** A project row and, unfolded, its worktrees hanging from the project's rail. */
function TerminalLauncherProjectRows({
  project,
  branchId,
  busy,
  onOpenNew,
  onToggle,
}: {
  project: TerminalLauncherProjectView;
  branchId: string;
  busy: boolean;
  onOpenNew: (destination: TerminalLauncherDestination) => void;
  onToggle: () => void;
}) {
  const { destination, expanded, shown, worktrees } = project;
  const count = worktrees.length;
  return (
    <>
      <CommandItem
        aria-controls={expanded ? branchId : undefined}
        aria-expanded={count > 0 ? expanded : undefined}
        data-testid={`terminal-launcher-new-${destination.value}`}
        disabled={busy || destination.unavailableReason !== null || !project.matches}
        onSelect={() => onOpenNew(destination)}
        value={terminalLauncherNewValue(destination.value)}
      >
        <Plus aria-hidden="true" className="text-subtle" />
        <span className="min-w-0 truncate">{destination.label}</span>
        {destination.unavailableReason ? (
          <span className="ms-auto ps-3 text-micro text-subtle">
            {destination.unavailableReason}
          </span>
        ) : null}
        {count > 0 ? (
          // The fold control: a pointer click, or Tab then Enter/Space (a search keeps ←/→ for
          // the caret). Its Enter folds here and never reaches the list, which would open a terminal.
          <button
            aria-label={`${expanded ? "Hide" : "Show"} ${count} ${count === 1 ? "worktree" : "worktrees"} of ${destination.label}`}
            className="ms-auto inline-flex h-5 shrink-0 items-center gap-1 rounded-pill px-1.5 font-mono text-micro text-subtle tabular-nums transition-colors duration-fast ease-out outline-none hover:bg-line-strong hover:text-fg focus-visible:shadow-focus-ring motion-reduce:transition-none"
            data-testid={`terminal-launcher-fold-${destination.value}`}
            onClick={event => {
              event.stopPropagation();
              onToggle();
            }}
            onKeyDown={event => {
              if (event.key === "Enter") event.stopPropagation();
            }}
            onPointerDown={event => event.preventDefault()}
            type="button"
          >
            <GitBranch aria-hidden="true" className="size-3" />
            {count}
            <ChevronRight
              aria-hidden="true"
              className={cn(
                "size-3.5 transition-transform duration-base ease-out motion-reduce:transition-none",
                expanded && "rotate-90"
              )}
            />
          </button>
        ) : null}
      </CommandItem>
      {expanded ? (
        // The rail continues under the project's leading mark.
        <TreeBranch className="ms-1.25" id={branchId}>
          {shown.map(worktree => (
            <TreeBranchNode
              key={worktree.value}
              render={
                <CommandItem
                  data-testid={`terminal-launcher-new-${worktree.value}`}
                  disabled={busy}
                  onSelect={() => onOpenNew(worktree)}
                  value={terminalLauncherNewValue(worktree.value)}
                />
              }
            >
              <GitBranch aria-hidden="true" className="text-subtle" />
              <span className="min-w-0 truncate">{worktree.label}</span>
            </TreeBranchNode>
          ))}
        </TreeBranch>
      ) : null}
    </>
  );
}
