/** The Sessions app's window: its rail of project groups beside the session pane. */
import { protoHref } from "@compozy/proto";
import { FolderPlus, MessageSquare, MessageSquarePlus, Plus, UserPlus } from "lucide-react";
import { useState, type ReactNode } from "react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Eyebrow,
  Icon,
  SearchInput,
  type TopbarSlotValue,
} from "@compozy/ui";

import { SessionViewToggle } from "@/systems/session/components/session-board/session-view-toggle";
import { SessionListBranch } from "@/systems/session/components/session-list/session-list-branch";
import {
  BRANCH_NODE,
  BRANCH_RAIL,
} from "@/systems/session/components/session-list/session-list-branch-rail";
import { SessionListBranchToggle } from "@/systems/session/components/session-list/session-list-branch-toggle";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SessionListToolbar } from "@/systems/session/components/session-list/session-list-toolbar";
import { SessionListTotals } from "@/systems/session/components/session-list/session-list-totals";
import { SessionPanelToggle } from "@/systems/session/components/session-panel-toggle";
import { SessionPrimaryAction } from "@/systems/session/components/session-primary-action";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import { SessionRowContextMenu } from "@/systems/session/components/session-row-context-menu";
import { childSessionSignalState } from "@/systems/session/lib/session-hierarchy";
import { SESSION_SIDEBAR_DEFAULT_WIDTH } from "@/systems/session/lib/session-sidebar-width";
import { sessionSubagentCounts } from "@/systems/session/lib/session-subagent-summary";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "@/systems/session/mocks/session-row-fixtures";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import { AgentPicker, type AgentPickerProps } from "./add-pickers";
import { AgentAvatar } from "./agent-avatar";
import { DraftShell, type DraftTab } from "./draft-shell";
import { type DraftInstance } from "./fixtures";
import { StaysOnRow, type StaysOnRowProps } from "./stays-on-row";
import { boardSession, railTotals, SPAWNED_BY } from "./sessions-fixtures";

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
  branch,
}: {
  id: string;
  current?: boolean;
  badge?: boolean;
  /** The fold toggle a row with child sessions carries on its meta line. */
  branch?: ReactNode;
}) {
  const session = boardSession(id);
  const creator = badge ? SPAWNED_BY[id] : undefined;
  return (
    <SessionListRowView
      current={current}
      onSelect={() => undefined}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      rowActions={<SessionRowActions actions={actions} session={session} />}
      runtime={boardRuntimeFixture(session)}
      session={session}
      sessionActions={actions}
      testIdPrefix="draft"
      trailing={
        branch || creator ? (
          <>
            {creator ? <CreatorBadge instance={creator} /> : null}
            {branch}
          </>
        ) : undefined
      }
      wrapRow={row => <SessionRowContextMenu actions={actions} render={row} session={session} />}
    />
  );
}

/** A root row and the child sessions that fold under it, with the real toggle and elbows. */
function RailThread({
  id,
  childIds,
  badges,
  currentRow,
}: {
  id: string;
  childIds: readonly string[];
  badges?: boolean;
  currentRow?: string;
}) {
  const [open, setOpen] = useState(true);
  const session = boardSession(id);
  const branchId = `draft-branch-${id}`;
  return (
    <div>
      <RailRow
        badge={badges}
        branch={
          <SessionListBranchToggle
            branchId={branchId}
            childCount={childIds.length}
            childSignal={open ? null : childSessionSignalState(childIds.map(boardSession))}
            onChange={next => setOpen(next !== null)}
            segment={open ? "children" : null}
            session={session}
            subagentCounts={sessionSubagentCounts(session)}
            testIdPrefix="draft"
          />
        }
        current={id === currentRow}
        id={id}
      />
      <div id={branchId}>
        <SessionListBranch
          render={() => (
            <div className={BRANCH_RAIL}>
              {childIds.map(child => (
                <div className={BRANCH_NODE} key={child}>
                  <RailRow badge={badges} current={child === currentRow} id={child} />
                </div>
              ))}
            </div>
          )}
          segment={open ? "children" : null}
        />
      </div>
    </div>
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
          <Icon as={FolderPlus} size="sm" />
          New folder
        </DropdownMenuItem>
        <DropdownMenuItem>
          <Icon as={UserPlus} size="sm" />
          <span className="flex flex-col">
            Add stays-on agent
            <span className="text-eyebrow text-subtle">Stays here and checks in on its own</span>
          </span>
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function ProjectAddPicker({
  name,
  picker,
}: {
  name: string;
  picker: Omit<AgentPickerProps, "name" | "trigger">;
}) {
  return (
    <AgentPicker
      {...picker}
      name={name}
      trigger={
        <Button
          aria-label={`Add to ${name}`}
          className="shrink-0"
          size="icon-xs"
          type="button"
          variant="quiet"
        >
          <Icon as={Plus} size="sm" />
        </Button>
      }
    />
  );
}

export interface RailGroup {
  name: string;
  count: number;
  rows?: readonly string[];
  /** Child sessions folded under a root row, by root id. */
  threads?: Readonly<Record<string, readonly string[]>>;
  instances?: readonly DraftInstance[];
  collapsed?: boolean;
  menuOpen?: boolean;
  /** Holds S16's agent picker open on this project's `+`. */
  picker?: Omit<AgentPickerProps, "name" | "trigger">;
  rowProps?: Partial<StaysOnRowProps>;
  badges?: boolean;
  currentRow?: string;
}

/** A closed project still shows who wants you, by face rather than a dot. */
function CollapsedSignal({ instances }: { instances: readonly DraftInstance[] }) {
  const waiting = instances.filter(instance => instance.state === "needs-you");
  const lead = waiting[0];
  if (!lead) return null;
  return (
    <span className="flex items-center gap-1">
      <AgentAvatar
        appearance={lead.agent.appearance}
        label={`${lead.agent.label} needs you`}
        size={20}
        state="needs-you"
        surfaceClassName="ring-sunken"
      />
      {waiting.length > 1 ? (
        <span className="text-micro text-muted">+{waiting.length - 1}</span>
      ) : null}
    </span>
  );
}

function Group({ group }: { group: RailGroup }) {
  const instances = group.instances ?? [];
  return (
    <section>
      <div className="flex min-h-7 min-w-0 items-center gap-1 rounded-sm">
        <SessionListGroupToggle
          collapsed={Boolean(group.collapsed)}
          count={group.count}
          identity={
            <WorkspaceBadge identity={{ kind: "project", name: group.name }} variant="chrome" />
          }
          signal={<CollapsedSignal instances={instances} />}
        />
        {group.picker ? (
          <ProjectAddPicker name={group.name} picker={group.picker} />
        ) : (
          <ProjectAddMenu name={group.name} open={group.menuOpen} />
        )}
      </div>
      {group.collapsed ? null : (
        <div className="flex flex-col gap-0.5 pl-2.5">
          <StaysOnRow
            className="px-1 pt-1 pb-1.5"
            instances={instances}
            scopeName={group.name}
            {...group.rowProps}
          />
          {(group.rows ?? []).map(id => {
            const childIds = group.threads?.[id];
            return childIds ? (
              <RailThread
                badges={group.badges}
                childIds={childIds}
                currentRow={group.currentRow}
                id={id}
                key={id}
              />
            ) : (
              <RailRow badge={group.badges} current={id === group.currentRow} id={id} key={id} />
            );
          })}
        </div>
      )}
    </section>
  );
}

function railRowIds({ groups, flatRows }: Pick<SessionRailProps, "groups" | "flatRows">) {
  if (flatRows) return flatRows;
  return groups.flatMap(group =>
    (group.rows ?? []).flatMap(id => [id, ...(group.threads?.[id] ?? [])])
  );
}

export interface SessionRailProps {
  groups: readonly RailGroup[];
  /** Global instances above all projects; flat mode passes the merged row here. */
  top?: readonly DraftInstance[];
  topProps?: Partial<StaysOnRowProps>;
  flatRows?: readonly string[];
  flatBadges?: boolean;
  currentRow?: string;
}

function GlobalRow({
  instances,
  ...props
}: { instances: readonly DraftInstance[] } & Partial<StaysOnRowProps>) {
  return (
    <StaysOnRow
      className="px-3 pb-2"
      instances={instances}
      label={<Eyebrow className="pr-1.5 text-faint">Global</Eyebrow>}
      scopeName="Global"
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
      className="flex h-full shrink-0 flex-col overflow-hidden bg-sunken pt-1.5"
      style={{ width: SESSION_SIDEBAR_DEFAULT_WIDTH }}
    >
      <SessionListToolbar
        groupBy={flatRows ? "none" : "project"}
        groupByOptions={["none", "project"]}
        onGroupByChange={() => undefined}
        onNewFolder={() => undefined}
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
      <div className="flex min-h-0 flex-1 flex-col gap-1 overflow-y-auto px-2.5 pt-0.5">
        {flatRows
          ? flatRows.map(id => (
              <RailRow badge={flatBadges} current={id === currentRow} id={id} key={id} />
            ))
          : groups.map(group => <Group group={group} key={group.name} />)}
      </div>
      <SessionListTotals
        testIdPrefix="draft-rail"
        totals={railTotals(railRowIds({ groups, flatRows }))}
      />
    </aside>
  );
}

/** The real head's stop, for a session that is running. */
export function DraftStopAction() {
  return (
    <SessionPrimaryAction
      controlsBusy={false}
      isStopping={false}
      isUnarchiving={false}
      onStop={() => undefined}
      onUnarchive={() => undefined}
      showStopAction
      showUnarchiveAction={false}
    />
  );
}

/** Leaves this list for the board, or the board for this list, as the real toggle does. */
export function DraftViewToggle({ value }: { value: "list" | "kanban" }) {
  return (
    <SessionViewToggle
      onValueChange={view =>
        window.location.assign(
          protoHref("stays-on-sessions", view === "list" ? "sidebar-rows" : "kanban-rows")
        )
      }
      value={value}
    />
  );
}

export interface SessionsWindowProps {
  rail: SessionRailProps;
  title: ReactNode;
  glyph?: ReactNode;
  /** The window's project, named before the title. */
  project?: string;
  /** The session's own head; the window adds the view toggle and the two panel toggles. */
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
  project = "compozy",
  topbar,
  tabs,
  activeTabId,
  sessionBadge,
  overlay,
  children,
}: SessionsWindowProps) {
  const [railOpen, setRailOpen] = useState(true);
  return (
    <DraftShell
      activeTabId={activeTabId}
      app="sessions"
      glyph={glyph ?? <MessageSquare />}
      leading={<WorkspaceBadge identity={{ kind: "project", name: project }} variant="chrome" />}
      overlay={overlay}
      sessionBadge={sessionBadge}
      tabs={tabs}
      title={title}
      topbar={{
        ...topbar,
        actions: (
          <>
            {topbar?.actions}
            <SessionPanelToggle
              onToggle={() => setRailOpen(!railOpen)}
              open={railOpen}
              panel="sidebar"
            />
            <SessionPanelToggle onToggle={() => undefined} open={false} panel="inspector" />
          </>
        ),
        overflow: <DraftViewToggle value="list" />,
      }}
    >
      <div className="flex min-h-0 min-w-0 flex-1">
        {railOpen ? <SessionRail {...rail} /> : null}
        <div className="flex min-w-0 flex-1 flex-col border-l border-line">{children}</div>
      </div>
    </DraftShell>
  );
}
