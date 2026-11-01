/** The Sessions app's window: its rail of project groups beside the session pane. */
import { ChevronRight, MessageSquare, MessageSquarePlus, Plus, UserPlus } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Icon,
  SearchInput,
  StatusDot,
  cn,
  type TopbarSlotValue,
} from "@compozy/ui";

import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SessionListToolbar } from "@/systems/session/components/session-list/session-list-toolbar";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "@/systems/session/mocks/session-row-fixtures";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { AgentAvatar } from "./agent-avatar";
import { DraftShell, type DraftTab } from "./draft-shell";
import { type DraftInstance } from "./fixtures";
import { PersistentRow, type PersistentRowProps } from "./persistent-row";
import { boardSession, SPAWNED_BY } from "./sessions-fixtures";

const actions = sessionLifecycleActionsFixture();

function CreatorBadge({ instance }: { instance: DraftInstance }) {
  return (
    <AgentAvatar
      appearance={instance.agent.appearance}
      label={`Started by ${instance.agent.label}`}
      quietIdle
      size={16}
      surfaceClassName="ring-sunken"
    />
  );
}

export function RailRow({
  id,
  current = false,
  badge = false,
}: {
  id: string;
  current?: boolean;
  badge?: boolean;
}) {
  const session = boardSession(id);
  const creator = badge ? SPAWNED_BY[id] : undefined;
  return (
    <SessionListRowView
      current={current}
      onSelect={() => undefined}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      runtime={boardRuntimeFixture(session)}
      session={session}
      sessionActions={actions}
      showActions={false}
      testIdPrefix="draft"
      trailing={creator ? <CreatorBadge instance={creator} /> : undefined}
    />
  );
}

function ProjectAddMenu({ name, open = false }: { name: string; open?: boolean }) {
  return (
    <DropdownMenu defaultOpen={open}>
      <DropdownMenuTrigger
        aria-label={`Add to ${name}`}
        render={<Button className="shrink-0" size="icon-xs" type="button" variant="quiet" />}
      >
        <Icon as={Plus} size="sm" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-52">
        <DropdownMenuItem>
          <Icon as={MessageSquarePlus} size="sm" />
          New session
        </DropdownMenuItem>
        <DropdownMenuItem>
          <Icon as={UserPlus} size="sm" />
          Add persistent agent
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

export interface RailGroup {
  name: string;
  count: number;
  rows?: readonly string[];
  instances?: readonly DraftInstance[];
  collapsed?: boolean;
  signal?: boolean;
  menuOpen?: boolean;
  rowProps?: Partial<PersistentRowProps>;
  badges?: boolean;
  currentRow?: string;
}

function Group({ group }: { group: RailGroup }) {
  const instances = group.instances ?? [];
  return (
    <div className="flex flex-col gap-0.5">
      <div className="flex min-h-7 min-w-0 items-center gap-1 rounded-sm">
        <span className="flex min-h-7 min-w-0 flex-1 items-center gap-2 rounded-sm px-2 py-1">
          <Icon
            as={ChevronRight}
            className={cn("shrink-0 text-faint", !group.collapsed && "rotate-90")}
            size="sm"
          />
          <span className="flex min-w-0 flex-1 items-center">
            <WorkspaceBadge identity={{ kind: "project", name: group.name }} variant="chrome" />
          </span>
          {group.collapsed && group.signal ? (
            <StatusDot label="1 needs you" size="sm" tone="accent" />
          ) : null}
          <span className="font-mono text-micro text-faint">{group.count}</span>
        </span>
        <ProjectAddMenu name={group.name} open={group.menuOpen} />
      </div>
      {group.collapsed ? null : (
        <>
          <PersistentRow className="px-1.5 pb-1" instances={instances} {...group.rowProps} />
          {(group.rows ?? []).map(id => (
            <RailRow badge={group.badges} current={id === group.currentRow} id={id} key={id} />
          ))}
        </>
      )}
    </div>
  );
}

export interface SessionRailProps {
  groups: readonly RailGroup[];
  /** Global instances above all projects; flat mode passes the merged row here. */
  top?: readonly DraftInstance[];
  topProps?: Partial<PersistentRowProps>;
  flatRows?: readonly string[];
  flatBadges?: boolean;
  currentRow?: string;
}

function GlobalRow({
  instances,
  ...props
}: { instances: readonly DraftInstance[] } & Partial<PersistentRowProps>) {
  return (
    <PersistentRow
      className="px-3 pb-1.5"
      instances={instances}
      label={<span className="pr-1.5 text-micro text-faint">Global</span>}
      {...props}
    />
  );
}

export function SessionRail({
  groups,
  top,
  topProps,
  flatRows,
  flatBadges,
  currentRow,
}: SessionRailProps) {
  return (
    <aside
      aria-label="Sessions"
      className="flex h-full w-80 shrink-0 flex-col overflow-hidden bg-sunken pt-1.5"
    >
      <SessionListToolbar
        groupBy={flatRows ? "none" : "project"}
        groupByOptions={["none", "project"]}
        onGroupByChange={() => undefined}
        onNewSession={() => undefined}
        onShelfChange={() => undefined}
        onShowEmptyProjectsChange={() => undefined}
        onSortChange={() => undefined}
        shelf="active"
        showEmptyProjects={false}
        sort="created"
        testIdPrefix="draft-rail"
      />
      <div className="px-3 pb-1.5">
        <SearchInput
          aria-label="Filter sessions"
          containerClassName="min-w-0"
          onChange={() => undefined}
          placeholder="Filter sessions…"
          value=""
        />
      </div>
      {top && top.length > 0 ? <GlobalRow instances={top} {...topProps} /> : null}
      <div className="flex min-h-0 flex-1 flex-col gap-1.5 overflow-y-auto px-2.5 pt-0.5">
        {flatRows
          ? flatRows.map(id => (
              <RailRow badge={flatBadges} current={id === currentRow} id={id} key={id} />
            ))
          : groups.map(group => <Group group={group} key={group.name} />)}
      </div>
    </aside>
  );
}

export interface SessionsWindowProps {
  rail: SessionRailProps;
  title: ReactNode;
  glyph?: ReactNode;
  topbar?: TopbarSlotValue;
  tabs?: readonly DraftTab[];
  activeTabId?: string;
  sessionBadge?: number;
  overlay?: ReactNode;
  children: ReactNode;
}

export function SessionsWindow({
  rail,
  title,
  glyph,
  topbar,
  tabs,
  activeTabId,
  sessionBadge,
  overlay,
  children,
}: SessionsWindowProps) {
  return (
    <DraftShell
      activeTabId={activeTabId}
      app="sessions"
      glyph={glyph ?? <MessageSquare />}
      overlay={overlay}
      sessionBadge={sessionBadge}
      tabs={tabs}
      title={title}
      topbar={topbar}
    >
      <div className="flex min-h-0 min-w-0 flex-1">
        <SessionRail {...rail} />
        <div className="flex min-w-0 flex-1 flex-col border-l border-line">{children}</div>
      </div>
    </DraftShell>
  );
}
