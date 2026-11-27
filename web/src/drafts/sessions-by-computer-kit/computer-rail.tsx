/**
 * The Sessions rail over several computers. Together merges every computer into the window's
 * grouping; Separate gives each computer its own section with the same grouping inside; a
 * window scoped to one computer shows that computer's list with no section header.
 */
import type { ReactNode } from "react";

import { SearchInput, cn } from "@compozy/ui";

import { SessionListSelectionBar } from "@/systems/session/components/session-list/session-list-selection-bar";
import { SessionListToolbar } from "@/systems/session/components/session-list/session-list-toolbar";
import { SessionListTotals } from "@/systems/session/components/session-list/session-list-totals";
import { ScopedComputerError } from "@/systems/computers/components/scoped-computer-status";
import {
  SessionListComputerContext,
  type SessionRowComputerInfo,
} from "@/systems/session/lib/session-list-computer";
import type {
  SessionListComputerLayout,
  SessionListGroupBy,
} from "@/systems/session/lib/session-list-preferences";
import type { SessionSelectionVerbs } from "@/systems/session/lib/session-selection-verbs";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
import type { SessionPayload } from "@/systems/session/types";

import { byProject, matches, totalsOf, type RailComputer, type RailLayout } from "./model";
import { ComputerBody, ComputerSection, ProjectGroup, Row } from "./rail-tree";
import { useOpenOnMount } from "./hooks/use-open-on-mount";
import { RailLookContext, type RailLook } from "./rail-look";

const noop = () => undefined;

function narrow(computer: RailComputer, query: string): RailComputer {
  if (query === "") return computer;
  return { ...computer, sessions: computer.sessions.filter(session => matches(session, query)) };
}

/** Together: one group per project and computer, then the computers that could not answer. */
function TogetherBody({
  computers,
  groupBy,
}: {
  computers: readonly RailComputer[];
  groupBy: SessionListGroupBy;
}) {
  const answered = computers.filter(
    computer => computer.read === "ok" || computer.read === "offline"
  );
  const failed = computers.filter(computer => computer.read === "failed");
  if (groupBy !== "project") {
    const info = new Map<string, SessionRowComputerInfo>();
    for (const computer of answered) {
      if (computer.key === "local") continue;
      for (const session of computer.sessions) {
        info.set(session.id, {
          id: computer.key,
          name: computer.name,
          platform: computer.computer.platform,
          iconKind: computer.computer.iconKind,
          offline: computer.read === "offline",
        });
      }
    }
    const rows = answered.flatMap(computer => computer.sessions);
    return (
      <SessionListComputerContext value={info}>
        {rows.map(session => (
          <Row key={session.id} session={session} />
        ))}
      </SessionListComputerContext>
    );
  }
  const groups = byProject(answered.flatMap(computer => computer.sessions)).flatMap(
    ([workspaceId]) =>
      answered.flatMap(computer => {
        const sessions = computer.sessions.filter(session => session.workspace_id === workspaceId);
        return sessions.length > 0 ? [{ computer, workspaceId, sessions }] : [];
      })
  );
  return (
    <>
      {groups.map(group => (
        <ProjectGroup
          computer={group.computer}
          key={`${group.computer.key}:${group.workspaceId}`}
          named
          sessions={group.sessions}
          workspaceId={group.workspaceId}
        />
      ))}
      {failed.map(computer => (
        <ScopedComputerError
          className="mx-1 mt-1"
          computerId={computer.key}
          computerName={computer.name}
          key={computer.key}
          onRetry={noop}
        />
      ))}
    </>
  );
}

export interface ComputerRailProps {
  layout: RailLayout;
  groupBy: SessionListGroupBy;
  onGroupByChange: (next: SessionListGroupBy) => void;
  onLayoutChange: (next: SessionListComputerLayout) => void;
  /** The computers in scope, this computer first. */
  computers: readonly RailComputer[];
  look: RailLook;
  /** A control to click on mount: View options, a `+`, the selection menu. */
  openOnMount?: string | readonly string[];
  /** Above the rows: a notice about the scope. */
  notice?: ReactNode;
  /** Replaces the toolbar while rows are selected. */
  selection?: SessionSelectionVerbs;
}

export function ComputerRail({
  layout,
  groupBy,
  onGroupByChange,
  onLayoutChange,
  computers,
  look,
  openOnMount,
  notice,
  selection,
}: ComputerRailProps) {
  useOpenOnMount(openOnMount);
  // A search hides the computers with no match; without one, every computer keeps its header.
  const shown = computers.flatMap(computer => {
    const narrowed = narrow(computer, look.query);
    return look.query === "" || narrowed.sessions.length > 0 ? [narrowed] : [];
  });
  const rows: SessionPayload[] = shown.flatMap(computer => computer.sessions);
  const scoped = layout === "scoped";
  const uncounted = computers.filter(
    computer =>
      computer.read !== "ok" && computer.read !== "offline" && computer.read !== "needs_pro"
  );
  let body: ReactNode;
  if (scoped) {
    body = shown[0] ? <ComputerBody computer={shown[0]} groupBy={groupBy} /> : null;
  } else if (layout === "separate") {
    body = shown.map(computer => (
      <ComputerSection computer={computer} groupBy={groupBy} key={computer.key} />
    ));
  } else {
    body = <TogetherBody computers={shown} groupBy={groupBy} />;
  }
  return (
    <aside
      aria-label="Sessions"
      className="flex h-full shrink-0 flex-col overflow-hidden bg-sunken pt-1.5"
      style={{ width: SESSION_SIDEBAR_DEFAULT_WIDTH }}
    >
      {selection ? (
        <SessionListSelectionBar
          allSelected={false}
          hiddenByFilter={0}
          onClear={noop}
          onSelectAll={noop}
          testIdPrefix="draft-rail"
          verbs={selection}
        />
      ) : (
        <SessionListToolbar
          computerLayout={
            scoped
              ? undefined
              : {
                  value: layout === "separate" ? "separate" : "together",
                  onValueChange: onLayoutChange,
                }
          }
          foldAll={null}
          groupBy={groupBy}
          groupByOptions={["none", "project", "profile"]}
          newFolderUnavailable={
            scoped ? null : "Folders live on one computer. Use the + of a computer or project."
          }
          onFoldAllChange={noop}
          onGroupByChange={onGroupByChange}
          onNewFolder={scoped ? noop : undefined}
          onNewSession={noop}
          onShelfChange={noop}
          onShowAutomationChange={noop}
          onShowEmptyProjectsChange={noop}
          onSortChange={noop}
          shelf="active"
          showAutomation={false}
          showEmptyProjects={false}
          sort={scoped ? "manual" : "created"}
          testIdPrefix="draft-rail"
        />
      )}
      <div className="px-3 pb-1.5">
        <SearchInput
          aria-label="Filter sessions"
          containerClassName="min-w-0"
          onChange={noop}
          placeholder="Filter sessions…"
          value={look.query}
        />
      </div>
      <RailLookContext value={look}>
        {notice}
        <div
          className={cn(
            "flex min-h-0 flex-1 flex-col overflow-y-auto px-2.5 pt-0.5 pb-2",
            layout === "separate"
              ? "gap-2 [&>section+section]:border-t [&>section+section]:border-line-soft [&>section+section]:pt-2"
              : "gap-1"
          )}
        >
          {body}
        </div>
      </RailLookContext>
      <SessionListTotals testIdPrefix="draft-rail" totals={totalsOf(rows)} />
      {uncounted.length > 0 && !scoped ? (
        <p className="m-0 px-4 pb-2 text-micro text-faint">
          Not counted: {uncounted.map(computer => computer.name).join(", ")}
        </p>
      ) : null}
    </aside>
  );
}
