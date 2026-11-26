import { Button, cn, Eyebrow, SearchInput } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import type { RemoteSessionRowEntry } from "@/systems/computers/mocks/remote-computers-fixtures-sessions";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListToolbar } from "@/systems/session/components/session-list/session-list-toolbar";
import { SessionListTotals } from "@/systems/session/components/session-list/session-list-totals";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { useRemoteSessionsSidebar } from "./hooks/use-remote-sessions-sidebar";
import {
  RemoteSessionRow,
  ScopedComputerError,
  ScopedComputerSkeleton,
  type PerComputerErrorBlock,
  type PerComputerLoadingBlock,
  type PerComputerPaginationBlock,
} from "./remote-sessions-sidebar-parts";

export type { PerComputerErrorBlock, PerComputerLoadingBlock, PerComputerPaginationBlock };

export interface RemoteSessionsSidebarProps {
  sessions: readonly RemoteSessionRowEntry[];
  activeSessionId?: string | null;
  onSelectSession?: (session: RemoteSessionRowEntry) => void;
  /** Initial grouping mode (`none` = unified merged list; `project` = `<project> · <computer>`). */
  initialGroupBy?: "none" | "project";
  /** Computers whose initial session page is still loading (scoped skeletons only). */
  loadingComputers?: readonly PerComputerLoadingBlock[];
  /** Computers whose session read failed (inline retry alert, never blanking other rows). */
  failedComputers?: readonly PerComputerErrorBlock[];
  /** Per-computer paging controls (`Load more from <name>`). */
  perComputerMore?: readonly PerComputerPaginationBlock[];
  onNewSession?: () => void;
  onOpenSearch?: () => void;
}

/**
 * Multi-computer Sessions sidebar (`_uiux.md` U5):
 * - Unified flat list with muted computer label on remote rows and dimmed `Offline` rows
 * - Group by `project · computer` keeping duplicate repo names separate per computer
 * - Scoped per-computer loading skeletons, inline retry alerts, and per-computer `Load more`
 */
export function RemoteSessionsSidebar({
  sessions,
  activeSessionId = null,
  onSelectSession,
  initialGroupBy = "none",
  loadingComputers = [],
  failedComputers = [],
  perComputerMore = [],
  onNewSession,
}: RemoteSessionsSidebarProps) {
  const sidebar = useRemoteSessionsSidebar(sessions, initialGroupBy);

  return (
    <aside
      aria-label="Sessions"
      className="flex h-full shrink-0 flex-col overflow-hidden bg-sunken pt-1.5"
      style={{ width: SESSION_SIDEBAR_DEFAULT_WIDTH }}
    >
      <SessionListToolbar
        foldAll={null}
        groupBy={sidebar.groupBy}
        groupByOptions={["none", "project"]}
        onFoldAllChange={mode => {
          if (mode === "collapsed") {
            sidebar.setCollapsedGroups(Object.fromEntries(sidebar.groups.map(g => [g.key, true])));
          } else {
            sidebar.setCollapsedGroups({});
          }
        }}
        onGroupByChange={next => sidebar.setGroupBy(next === "project" ? "project" : "none")}
        onNewSession={() => onNewSession?.()}
        onShelfChange={() => undefined}
        onShowAutomationChange={() => undefined}
        onShowEmptyProjectsChange={() => undefined}
        onSortChange={() => undefined}
        shelf="active"
        showAutomation={false}
        showEmptyProjects={false}
        sort="created"
        testIdPrefix="remote-rail"
      />

      <div className="px-3 pb-1.5">
        <SearchInput
          aria-label="Filter sessions"
          containerClassName="min-w-0"
          onChange={sidebar.setQuery}
          placeholder="Filter sessions or computers…"
          value={sidebar.query}
        />
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-2.5 pt-0.5 pb-2">
        {sidebar.groupBy === "none" ? (
          <div className="flex flex-col gap-0.5">
            {sidebar.filtered.map(entry => (
              <RemoteSessionRow
                current={entry.id === activeSessionId}
                entry={entry}
                key={entry.id}
                onSelect={() => onSelectSession?.(entry)}
              />
            ))}
          </div>
        ) : (
          <div className="flex flex-col gap-1.5">
            {sidebar.groups.map(group => {
              const collapsed = Boolean(sidebar.collapsedGroups[group.key]);
              return (
                <section
                  className={cn(group.offline && "opacity-70")}
                  data-testid={`project-computer-group-${group.project}-${group.computerName}`}
                  key={group.key}
                >
                  <div className="flex min-h-7 min-w-0 items-center gap-1 rounded-sm">
                    <SessionListGroupToggle
                      collapsed={collapsed}
                      count={group.rows.length}
                      identity={
                        <span className="flex min-w-0 items-center gap-1.5">
                          <WorkspaceBadge
                            identity={{ kind: "project", name: group.project }}
                            variant="chrome"
                          />
                          <span aria-hidden="true" className="text-faint">
                            ·
                          </span>
                          <span className="inline-flex min-w-0 items-center gap-1 truncate text-eyebrow text-muted-foreground">
                            {!group.isLocal ? (
                              <ComputerPlatformIcon
                                className="size-3 shrink-0"
                                iconKind={group.iconKind}
                                platform={group.platform}
                                size="sm"
                                tone="muted"
                              />
                            ) : null}
                            <span className="truncate">{group.computerName}</span>
                            {group.offline ? (
                              <span className="shrink-0 text-badge text-faint">(Offline)</span>
                            ) : null}
                          </span>
                        </span>
                      }
                      onToggle={() =>
                        sidebar.setCollapsedGroups(prev => ({ ...prev, [group.key]: !collapsed }))
                      }
                    />
                  </div>
                  {!collapsed ? (
                    <div className="flex flex-col gap-0.5 pl-2.5">
                      {group.rows.map(entry => (
                        <RemoteSessionRow
                          current={entry.id === activeSessionId}
                          entry={entry}
                          hideComputer
                          hideProject
                          key={entry.id}
                          onSelect={() => onSelectSession?.(entry)}
                        />
                      ))}
                    </div>
                  ) : null}
                </section>
              );
            })}
          </div>
        )}

        {perComputerMore.length > 0 ? (
          <div className="mt-1 flex flex-col gap-1 border-t border-line-soft pt-2">
            <Eyebrow className="px-2 text-faint">More by computer</Eyebrow>
            {perComputerMore.map(pager => {
              const loaded = sidebar.loadedMoreSet.has(pager.computerId);
              return (
                <Button
                  className="h-7 justify-between px-2 text-eyebrow text-muted-foreground"
                  disabled={loaded}
                  key={pager.computerId}
                  onClick={() => sidebar.setLoadedMoreIds(prev => [...prev, pager.computerId])}
                  size="xs"
                  type="button"
                  variant="quiet"
                >
                  <span className="truncate">
                    {loaded
                      ? `Loaded more from ${pager.computerName}`
                      : `Load more from ${pager.computerName}`}
                  </span>
                  <span className="font-mono text-micro text-faint tabular-nums">
                    {loaded ? "0" : `+${pager.remainingCount}`}
                  </span>
                </Button>
              );
            })}
          </div>
        ) : null}

        {loadingComputers.map(block => (
          <ScopedComputerSkeleton block={block} key={block.computerId} />
        ))}

        {failedComputers.map(block => (
          <ScopedComputerError
            block={block}
            key={block.computerId}
            onRetry={() => {
              sidebar.setRetryingIds(prev => [...prev, block.computerId]);
            }}
            retrying={sidebar.retryingSet.has(block.computerId)}
          />
        ))}
      </div>

      <SessionListTotals testIdPrefix="remote-rail" totals={sidebar.totals} />
    </aside>
  );
}
