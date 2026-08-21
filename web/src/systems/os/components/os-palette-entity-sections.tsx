import { ArrowRightLeft, FolderGit2 } from "lucide-react";

import { CommandGroup, CommandItem, MonoId } from "@compozy/ui";

import type { OsPaletteEntities, OsPaletteWorktreeResult } from "../hooks/use-os-palette-entities";
import { OS_APP_DESCRIPTORS } from "../lib/app-catalog";
import {
  paletteGroupClass,
  paletteGroupFollowClass,
  paletteRowClass,
} from "../lib/palette-view-inset";

const GROUP_CLASS = `${paletteGroupClass} ${paletteGroupFollowClass}`;

function EntityOverflow({ shown, total }: { shown: number; total: number }) {
  if (total <= shown) return null;
  return <div className="px-3 py-1 text-micro text-faint">{`showing ${shown} of ${total}`}</div>;
}

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
          className={GROUP_CLASS}
          data-testid="os-palette-section-sessions"
          heading="Sessions"
        >
          {entities.sessions.map(session => (
            <CommandItem
              className={paletteRowClass}
              data-palette-row={`session:${session.sessionId}`}
              data-testid={`os-palette-session-${session.sessionId}`}
              forceMount
              key={session.sessionId}
              value={`session:${session.sessionId}`}
              onSelect={() => onOpenSession(session)}
            >
              <OS_APP_DESCRIPTORS.session.icon className="size-3.5 text-muted" />
              <span className="min-w-0 truncate leading-none">{session.title}</span>
              <span className="ml-auto shrink-0 text-micro text-subtle">
                {session.agentName}
                {session.workspaceLabel ? ` · ${session.workspaceLabel}` : ""}
              </span>
            </CommandItem>
          ))}
          <EntityOverflow shown={entities.sessions.length} total={entities.sessionTotal} />
        </CommandGroup>
      ) : null}

      {destination || entities.tabs.length === 0 ? null : (
        <CommandGroup
          className={GROUP_CLASS}
          data-testid="os-palette-section-tabs"
          heading="Go to tab"
        >
          {entities.tabs.map(tab => (
            <CommandItem
              className={paletteRowClass}
              data-palette-row={`tab:${tab.windowId}`}
              data-testid={`os-palette-tab-${tab.windowId}`}
              forceMount
              key={tab.windowId}
              value={`tab:${tab.windowId}`}
              onSelect={() => onGoToTab(tab.windowId)}
            >
              <ArrowRightLeft className="size-3.5 text-muted" />
              <span className="min-w-0 truncate leading-none">{tab.label}</span>
              {tab.needsInput ? (
                <span
                  className="inline-flex h-3.5 min-w-deck-badge shrink-0 items-center justify-center rounded-full bg-accent px-1 font-mono text-pill-group-badge leading-none font-bold text-accent-ink"
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
          <EntityOverflow shown={entities.tabs.length} total={entities.tabTotal} />
        </CommandGroup>
      )}

      {destination || entities.worktrees.length === 0 ? null : (
        <CommandGroup
          className={GROUP_CLASS}
          data-testid="os-palette-section-worktrees"
          heading="Worktrees"
        >
          {entities.worktrees.map(entry => (
            <CommandItem
              className={paletteRowClass}
              data-palette-row={`worktree:${entry.key}`}
              data-testid={`os-palette-worktree-${entry.key}`}
              forceMount
              key={entry.key}
              value={`worktree:${entry.key}`}
              onSelect={() => onSelectWorktree(entry)}
            >
              <FolderGit2 className="size-3.5 text-muted" />
              <span className="min-w-0 truncate leading-none">{entry.name}</span>
              <MonoId className="ml-auto" preserveCase size="sm" value={entry.branch} />
              {entry.workspaceLabel ? (
                <span className="shrink-0 text-micro text-faint">{entry.workspaceLabel}</span>
              ) : null}
            </CommandItem>
          ))}
          <EntityOverflow shown={entities.worktrees.length} total={entities.worktreeTotal} />
        </CommandGroup>
      )}
    </>
  );
}
