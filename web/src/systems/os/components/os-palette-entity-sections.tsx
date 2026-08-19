import { ArrowRightLeft, FolderGit2 } from "lucide-react";

import { CommandGroup, CommandItem, MonoId, cn } from "@compozy/ui";

import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import type { OsPaletteEntities } from "../hooks/use-os-palette-entities";
import type { OsPaletteWorktreeResult } from "../hooks/use-os-palette-entities";

const GROUP_CLASS = "mt-2 border-t border-line-soft px-2 pt-2 pb-0.5";
const HEADING_CLASS = "**:[[cmdk-group-heading]]:text-faint";
const ROW_CLASS = "h-12 gap-3 px-3";

export interface OsPaletteEntitySectionsProps {
  entities: OsPaletteEntities;
  /** Destination mode offers sessions as tab targets and hides the rest. */
  destination: boolean;
  onOpenSession: (session: OsPaletteEntities["sessions"][number]) => void;
  onGoToTab: (windowId: string) => void;
  onSelectWorktree: (entry: OsPaletteWorktreeResult) => void;
}

/**
 * Sessions, open tabs and ready worktrees.
 *
 * These are entities rather than commands, so they live beside the registry
 * projection instead of inside it — but they share its row grammar and its
 * dispatch discipline: a session row lands through the shared attention jump,
 * never through a second `userOpen` of its own (BR-20).
 */
export function OsPaletteEntitySections({
  entities,
  destination,
  onOpenSession,
  onGoToTab,
  onSelectWorktree,
}: OsPaletteEntitySectionsProps) {
  return (
    <>
      {entities.sessions.length > 0 ? (
        <CommandGroup
          className={cn(GROUP_CLASS, HEADING_CLASS)}
          data-testid="os-palette-section-sessions"
          heading="Sessions"
        >
          {entities.sessions.map(session => (
            <CommandItem
              className={ROW_CLASS}
              data-testid={`os-palette-session-${session.sessionId}`}
              forceMount
              key={session.sessionId}
              value={`session:${session.sessionId}`}
              onSelect={() => onOpenSession(session)}
            >
              <OS_APP_DESCRIPTORS.session.icon className="size-3.5 text-muted" />
              <span className="min-w-0 truncate">{session.title}</span>
              <span className="ml-auto shrink-0 text-micro text-subtle">
                {session.agentName}
                {session.workspaceLabel ? ` · ${session.workspaceLabel}` : ""}
              </span>
            </CommandItem>
          ))}
        </CommandGroup>
      ) : null}

      {destination || entities.tabs.length === 0 ? null : (
        <CommandGroup
          className={cn(GROUP_CLASS, HEADING_CLASS)}
          data-testid="os-palette-section-tabs"
          heading="Go to tab"
        >
          {entities.tabs.map(tab => (
            <CommandItem
              className={ROW_CLASS}
              data-testid={`os-palette-tab-${tab.windowId}`}
              forceMount
              key={tab.windowId}
              value={`tab:${tab.windowId}`}
              onSelect={() => onGoToTab(tab.windowId)}
            >
              <ArrowRightLeft className="size-3.5 text-muted" />
              <span className="min-w-0 truncate">{tab.label}</span>
              {tab.needsInput ? (
                <span
                  className="inline-flex h-3.5 min-w-deck-badge shrink-0 items-center justify-center rounded-full bg-accent px-1 font-mono text-[9px] leading-none font-bold text-accent-ink"
                  data-slot="os-palette-tab-attention"
                >
                  1
                </span>
              ) : null}
              <span className="ml-auto shrink-0 text-micro text-subtle">
                {tab.minimized ? "minimized · " : ""}
                {tab.desktopName}
              </span>
            </CommandItem>
          ))}
        </CommandGroup>
      )}

      {destination || entities.worktrees.length === 0 ? null : (
        <CommandGroup
          className={cn(GROUP_CLASS, HEADING_CLASS)}
          data-testid="os-palette-section-worktrees"
          heading="Worktrees"
        >
          {entities.worktrees.map(entry => (
            <CommandItem
              className={ROW_CLASS}
              data-testid={`os-palette-worktree-${entry.key}`}
              forceMount
              key={entry.key}
              value={`worktree:${entry.key}`}
              onSelect={() => onSelectWorktree(entry)}
            >
              <FolderGit2 className="size-3.5 text-muted" />
              <span className="min-w-0 truncate">{entry.name}</span>
              <MonoId className="ml-auto" preserveCase size="sm" value={entry.branch} />
              {entry.workspaceLabel ? (
                <span className="shrink-0 text-micro text-faint">{entry.workspaceLabel}</span>
              ) : null}
            </CommandItem>
          ))}
        </CommandGroup>
      )}
    </>
  );
}
