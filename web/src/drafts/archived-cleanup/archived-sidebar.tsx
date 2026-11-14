import { useState, type ReactNode } from "react";
import { Eraser } from "@compozy/ui/icons";

import { Button, SearchInput } from "@compozy/ui";

import { AgentIdentityCatalogContext } from "@/systems/agent/lib/agent-identity-context";
import { SessionListFolder } from "@/systems/session/components/session-list/session-list-folder";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SessionListToolbar } from "@/systems/session/components/session-list/session-list-toolbar";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import type { SessionListShelf } from "@/systems/session/lib/session-list-shelf";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
import { agentCatalog } from "@/systems/session/mocks/session-list-declutter-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "@/systems/session/mocks/session-row-fixtures";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import type { SessionPayload } from "@/systems/session/types";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

const actions = sessionLifecycleActionsFixture();
const noop = () => undefined;

export function ArchivedRow({ session }: { session: SessionPayload }) {
  return (
    <SessionListRowView
      onSelect={noop}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      rowActions={<SessionRowActions actions={actions} session={session} />}
      runtime={boardRuntimeFixture(session)}
      session={session}
      sessionActions={actions}
      testIdPrefix="archived-cleanup"
    />
  );
}

export function ArchivedFolder({
  name,
  count,
  rows,
}: {
  name: string;
  count: number;
  rows: readonly SessionPayload[];
}) {
  const [expanded, setExpanded] = useState(true);
  return (
    <SessionListFolder
      count={count}
      expanded={expanded}
      folder={{ id: name, name }}
      onDelete={noop}
      onRename={noop}
      onToggle={() => setExpanded(value => !value)}
      testIdPrefix="archived-cleanup"
    >
      {rows.map(session => (
        <ArchivedRow key={session.id} session={session} />
      ))}
    </SessionListFolder>
  );
}

/** A project group header of the All projects list, by Group by → Project. */
export function ArchivedProjectGroup({
  project,
  count,
  collapsed,
  children,
}: {
  project: string;
  count: number;
  collapsed: boolean;
  children?: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <SessionListGroupToggle
        collapsed={collapsed}
        count={count}
        identity={<WorkspaceBadge identity={{ kind: "project", name: project }} variant="chrome" />}
      />
      {collapsed ? null : <div className="flex flex-col gap-0.5 pl-2.5">{children}</div>}
    </div>
  );
}

/**
 * The Archived list's entry point (#1717): a quiet footer verb under the list, never a loud
 * call to action, and only on the Archived view. It names how much the archive holds so the
 * reason to clean up is visible without opening anything.
 */
export function ArchivedCleanupFooter({
  archivedCount,
  onCleanUp,
}: {
  archivedCount: number;
  onCleanUp: () => void;
}) {
  return (
    <footer
      aria-label="Archive"
      className="flex shrink-0 items-center gap-2 border-t border-line px-2.5 py-1.5"
      data-testid="archived-cleanup-footer"
    >
      <Button
        data-testid="archived-cleanup-open"
        disabled={archivedCount === 0}
        onClick={onCleanUp}
        size="sm"
        variant="quiet"
      >
        <Eraser />
        Clean up history…
      </Button>
      <span className="ml-auto pr-1.5 text-micro text-subtle">
        <span className="font-mono text-muted-foreground tabular-nums">
          {archivedCount.toLocaleString()}
        </span>{" "}
        archived
      </span>
    </footer>
  );
}

/**
 * The in-window sessions rail, assembled from the real list parts the way `SessionList` lays
 * them out (the live list reads the daemon, which prototypes never reach).
 */
export function ArchivedSidebar({
  shelf = "archive",
  children,
  footer,
}: {
  shelf?: SessionListShelf;
  children: ReactNode;
  footer?: ReactNode;
}) {
  const catalog = agentCatalog();
  return (
    <AgentIdentityCatalogContext value={() => catalog}>
      <aside
        aria-label="Sessions"
        className="flex h-dvh shrink-0 flex-col overflow-hidden border-r border-line bg-sunken pt-1.5"
        style={{ width: SESSION_SIDEBAR_DEFAULT_WIDTH }}
      >
        <SessionListToolbar
          allCollapsed={false}
          groupBy="none"
          groupByOptions={["none", "project"]}
          laterCount={3}
          onAllCollapsedChange={noop}
          onGroupByChange={noop}
          onNewSession={noop}
          onShelfChange={noop}
          onShowAutomationChange={noop}
          onShowEmptyProjectsChange={noop}
          onSortChange={noop}
          shelf={shelf}
          showAutomation={false}
          showEmptyProjects={false}
          sort="last_activity"
          testIdPrefix="archived-cleanup"
        />
        <div className="px-3 pb-1.5">
          <SearchInput
            aria-label="Filter sessions"
            containerClassName="min-w-0"
            onChange={noop}
            placeholder="Filter sessions…"
            value=""
          />
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2.5 pt-0.5">
          {children}
        </div>
        {footer}
      </aside>
    </AgentIdentityCatalogContext>
  );
}
