/**
 * The rows of a session list that knows its computers. A computer section holds one daemon's
 * list; inside it, project groups and folders work as they do today. A folder belongs to one
 * computer, so it is created and shown only inside that computer.
 */
import {
  Ban,
  RefreshCw,
  Sparkles,
  TriangleAlert,
  WifiOff,
  type LucideIcon,
} from "@compozy/ui/icons";
import { use, type ReactNode } from "react";

import { Button, Icon, Spinner, cn } from "@compozy/ui";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import {
  ScopedComputerError,
  ScopedComputerSkeleton,
} from "@/systems/computers/components/scoped-computer-status";
import { SessionListCreateMenu } from "@/systems/session/components/session-list/session-list-create-menu";
import { SessionListFolder } from "@/systems/session/components/session-list/session-list-folder";
import { GroupNote } from "@/systems/session/components/session-list/session-list-group-note";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import { SessionRowContextMenu } from "@/systems/session/components/session-row-context-menu";
import type { SessionListGroupBy } from "@/systems/session/lib/session-list-preferences";
import { SessionListBadgeContext } from "@/systems/session/lib/session-list-row-badge";
import { boardRuntimeFixture } from "@/systems/session/mocks/sessions-board-view-fixtures";
import {
  SESSION_ENTRY_PRELOAD_FIXTURE,
  sessionLifecycleActionsFixture,
} from "@/systems/session/mocks/session-row-fixtures";
import type { SessionPayload } from "@/systems/session/types";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

import {
  PROJECT_FOLDERS,
  SESSION_FOLDERS,
  projectName,
  type ProjectFolderFixture,
  type SessionFolderFixture,
} from "./fixtures";
import { byProject, leadingOf, type RailComputer } from "./model";

const actions = sessionLifecycleActionsFixture();
import { useFold } from "./hooks/use-fold";
import { RailLookContext } from "./rail-look";

export type { RailLook } from "./rail-look";

const noop = () => undefined;

export function Row({ session }: { session: SessionPayload }) {
  const look = use(RailLookContext);
  const selection = look.selected
    ? {
        selectedIds: look.selected,
        mode: true,
        toggle: noop,
        toggleRange: noop,
        selectOnly: noop,
      }
    : undefined;
  return (
    <SessionListRowView
      current={session.id === look.currentRow}
      dragging={session.id === look.dragging}
      onSelect={noop}
      preloadHandlers={SESSION_ENTRY_PRELOAD_FIXTURE}
      rowActions={<SessionRowActions actions={actions} session={session} />}
      runtime={boardRuntimeFixture(session)}
      selection={selection}
      session={session}
      sessionActions={actions}
      testIdPrefix="draft"
      wrapRow={row => <SessionRowContextMenu actions={actions} render={row} session={session} />}
    />
  );
}

/** Rows that do not sit under their project name it, as a flat All projects list does. */
const projectBadge = (session: SessionPayload) => ({
  kind: "project" as const,
  name: projectName(session.workspace_id),
});

export function FlatRows({ sessions }: { sessions: readonly SessionPayload[] }) {
  return (
    <SessionListBadgeContext value={projectBadge}>
      {sessions.map(session => (
        <Row key={session.id} session={session} />
      ))}
    </SessionListBadgeContext>
  );
}

/** A computer named after a project: the platform mark and the name, in the meta tone. */
export function ComputerTag({
  computer,
  bare = false,
}: {
  computer: RailComputer;
  bare?: boolean;
}) {
  return (
    <span className="flex min-w-0 items-center gap-1 text-eyebrow text-subtle">
      {bare ? null : <ComputerMark computer={computer} />}
      <span className="truncate">{computer.name}</span>
    </span>
  );
}

export function ComputerMark({ computer }: { computer: RailComputer }) {
  return (
    <ComputerPlatformIcon
      className="size-3 shrink-0"
      iconKind={computer.computer.iconKind}
      platform={computer.computer.platform}
      size="sm"
      tone="muted"
    />
  );
}

function SessionFolder({
  folder,
  sessions,
  readOnly,
}: {
  folder: SessionFolderFixture;
  sessions: readonly SessionPayload[];
  readOnly: boolean;
}) {
  const look = use(RailLookContext);
  const [open, toggle] = useFold(`folder:${folder.id}`);
  const ids = new Set(folder.sessionIds);
  const rows = sessions.filter(session => ids.has(session.id));
  if (look.query !== "" && rows.length === 0) return null;
  const { needsYou, working } = leadingOf(rows);
  return (
    <SessionListFolder
      count={rows.length}
      dropEdge={look.dropInto === folder.id ? "into" : null}
      expanded={open}
      folder={folder}
      needsYou={needsYou}
      onDelete={noop}
      onNewSession={readOnly ? undefined : noop}
      onRename={noop}
      onToggle={toggle}
      running={working}
      testIdPrefix="draft"
    >
      {rows.map(session => (
        <Row key={session.id} session={session} />
      ))}
    </SessionListFolder>
  );
}

export interface ProjectGroupProps {
  computer: RailComputer;
  workspaceId: string;
  sessions: readonly SessionPayload[];
  /**
   * Together: one group per project and computer. The computer's mark takes the chevron's cell
   * at rest, and its name follows the project.
   */
  named?: boolean;
}

/** One project on one computer: its folders first, then its loose rows. */
export function ProjectGroup({
  computer,
  workspaceId,
  sessions,
  named = false,
}: ProjectGroupProps) {
  const [open, toggle] = useFold(`project:${computer.key}:${workspaceId}`);
  const readOnly = computer.read !== "ok";
  const folders = SESSION_FOLDERS.filter(
    folder => folder.computerId === computer.key && folder.workspaceId === workspaceId
  );
  const filed = new Set(folders.flatMap(folder => folder.sessionIds));
  const name = projectName(workspaceId);
  const testId = `draft-group-${computer.key}-${workspaceId}`;
  return (
    <section data-testid={testId}>
      <div className="flex min-h-7 min-w-0 items-center gap-1 rounded-sm">
        <SessionListGroupToggle
          collapsed={!open}
          count={sessions.length}
          identity={
            <span className="flex min-w-0 items-center gap-1.5">
              <WorkspaceBadge
                className="font-medium text-fg"
                identity={{ kind: "project", name }}
                variant="chrome"
              />
              {named ? <ComputerTag bare computer={computer} /> : null}
            </span>
          }
          leading={leadingOf(sessions)}
          mark={named ? <ComputerMark computer={computer} /> : undefined}
          onToggle={toggle}
        />
        {readOnly ? null : (
          <SessionListCreateMenu
            className="text-subtle"
            label={{
              session: `New session in ${name} on ${computer.name}`,
              menu: `New session or folder in ${name} on ${computer.name}`,
            }}
            onNewFolder={noop}
            onNewSession={noop}
            size="icon-xs"
            testId={testId}
            variant="quiet"
          />
        )}
      </div>
      {open ? (
        <div className="flex flex-col gap-0.5 pl-2.5">
          {folders.map(folder => (
            <SessionFolder
              folder={folder}
              key={folder.id}
              readOnly={readOnly}
              sessions={sessions}
            />
          ))}
          {sessions.flatMap(session =>
            filed.has(session.id) ? [] : [<Row key={session.id} session={session} />]
          )}
        </div>
      ) : null}
    </section>
  );
}

function ProjectFolder({
  folder,
  computer,
  groups,
}: {
  folder: ProjectFolderFixture;
  computer: RailComputer;
  groups: readonly [string, SessionPayload[]][];
}) {
  const [open, toggle] = useFold(`folder:${folder.id}`);
  const rows = groups.flatMap(([, sessions]) => sessions);
  const { needsYou, working } = leadingOf(rows);
  return (
    <SessionListFolder
      count={rows.length}
      expanded={open}
      folder={folder}
      holds="projects"
      needsYou={needsYou}
      onDelete={noop}
      onRename={noop}
      onToggle={toggle}
      running={working}
      testIdPrefix="draft"
    >
      {groups.map(([workspaceId, sessions]) => (
        <ProjectGroup
          computer={computer}
          key={workspaceId}
          sessions={sessions}
          workspaceId={workspaceId}
        />
      ))}
    </SessionListFolder>
  );
}

/** One computer's projects: its project folders, then the projects outside a folder. */
export function ProjectTree({ computer }: { computer: RailComputer }) {
  const groups = byProject(computer.sessions);
  const folders = PROJECT_FOLDERS.filter(folder => folder.computerId === computer.key);
  const filed = new Set(folders.flatMap(folder => folder.workspaceIds));
  return (
    <>
      {folders.map(folder => {
        const held = new Set(folder.workspaceIds);
        const inside = groups.filter(([workspaceId]) => held.has(workspaceId));
        return inside.length > 0 ? (
          <ProjectFolder computer={computer} folder={folder} groups={inside} key={folder.id} />
        ) : null;
      })}
      {groups.flatMap(([workspaceId, sessions]) =>
        filed.has(workspaceId)
          ? []
          : [
              <ProjectGroup
                computer={computer}
                key={workspaceId}
                sessions={sessions}
                workspaceId={workspaceId}
              />,
            ]
      )}
    </>
  );
}

function Note({
  icon,
  tone = "subtle",
  action,
  children,
}: {
  icon?: LucideIcon;
  tone?: "subtle" | "warning";
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <GroupNote>
      {icon ? (
        <Icon
          as={icon}
          className={tone === "warning" ? "shrink-0 text-warning" : "shrink-0 text-faint"}
          size="sm"
        />
      ) : null}
      <span className="min-w-0 flex-1">{children}</span>
      {action}
    </GroupNote>
  );
}

const smallAction = "h-6 shrink-0 px-2 text-badge";

/** A computer's list, or why it has none: loading, failed, offline, plan, version or empty. */
export function ComputerBody({
  computer,
  groupBy,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
}) {
  const { name } = computer;
  switch (computer.read) {
    case "connecting":
      return (
        <ScopedComputerSkeleton className="mx-1" computerId={computer.key} computerName={name} />
      );
    case "failed":
      return (
        <ScopedComputerError
          className="mx-1"
          computerId={computer.key}
          computerName={name}
          onRetry={noop}
        />
      );
    case "offline_empty":
      return <Note icon={WifiOff}>{name} is offline. Its sessions show when it reconnects.</Note>;
    case "needs_pro":
      return (
        <Note
          action={
            <Button className={smallAction} size="xs" type="button" variant="secondary">
              Upgrade
            </Button>
          }
          icon={Sparkles}
        >
          Pro lists sessions from more than one remote computer.
        </Note>
      );
    case "incompatible":
      return (
        <Note
          action={
            <Button className={smallAction} size="xs" type="button" variant="secondary">
              Details
            </Button>
          }
          icon={TriangleAlert}
          tone="warning"
        >
          Update Compozy on {name} to list its sessions.
        </Note>
      );
    default:
      break;
  }
  if (computer.sessions.length === 0) return <Note>No sessions on {name} yet.</Note>;
  const rows =
    groupBy === "project" ? (
      <ProjectTree computer={computer} />
    ) : (
      <FlatRows sessions={computer.sessions} />
    );
  return (
    <div className={cn("flex flex-col gap-0.5", computer.read === "offline" && "opacity-70")}>
      {rows}
      {computer.more ? (
        <GroupNote>
          <Button size="sm" type="button" variant="ghost">
            Next sessions
          </Button>
        </GroupNote>
      ) : null}
    </div>
  );
}

function ComputerIdentity({ computer }: { computer: RailComputer }) {
  const quiet =
    computer.read === "offline" ||
    computer.read === "offline_empty" ||
    computer.read === "needs_pro";
  return (
    <span className="flex min-w-0 items-center gap-2">
      <ComputerPlatformIcon
        className="size-3.5 shrink-0"
        iconKind={computer.computer.iconKind}
        platform={computer.computer.platform}
        size="sm"
        tone="muted"
      />
      <span className="truncate text-small-body font-medium text-fg">{computer.name}</span>
      <span
        className={cn(
          "truncate text-eyebrow",
          computer.read === "incompatible" ? "text-warning" : quiet ? "text-faint" : "text-subtle"
        )}
      >
        {computer.detail}
      </span>
    </span>
  );
}

/** The header's end: create where the computer runs agents, Reconnect where it is away. */
function ComputerAction({
  computer,
  groupBy,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
}) {
  if (computer.read === "connecting") return <Spinner className="mr-1.5 size-3 text-faint" />;
  if (computer.read === "offline" || computer.read === "offline_empty") {
    return (
      <Button className={smallAction} size="xs" type="button" variant="secondary">
        <RefreshCw aria-hidden="true" className="size-3" />
        Reconnect
      </Button>
    );
  }
  if (computer.read !== "ok") return null;
  return (
    <SessionListCreateMenu
      className="text-subtle"
      folderLabel="New project folder"
      label={{
        session: `New session on ${computer.name}`,
        menu: `New session or project folder on ${computer.name}`,
      }}
      newFolderUnavailable={groupBy === "project" ? null : "Group by Project to add folders."}
      onNewFolder={groupBy === "project" ? noop : undefined}
      onNewSession={noop}
      size="icon-xs"
      testId={`draft-computer-${computer.key}`}
      variant="quiet"
    />
  );
}

/** One computer's section: its header, then its own list with the window's grouping inside. */
export function ComputerSection({
  computer,
  groupBy,
}: {
  computer: RailComputer;
  groupBy: SessionListGroupBy;
}) {
  const look = use(RailLookContext);
  const [open, toggle] = useFold(`computer:${computer.key}`);
  const refused = look.refusedOn === computer.key;
  const counted = computer.read === "ok" || computer.read === "offline";
  return (
    <section
      className={cn(
        "flex flex-col gap-0.5 rounded-md",
        refused && "cursor-not-allowed bg-surface-2/60"
      )}
      data-testid={`draft-computer-${computer.key}`}
    >
      <div className="sticky top-0 z-10 flex min-h-8 min-w-0 items-center gap-1 rounded-sm bg-sunken">
        <SessionListGroupToggle
          collapsed={!open}
          count={counted ? computer.sessions.length : null}
          identity={<ComputerIdentity computer={computer} />}
          leading={leadingOf(computer.sessions)}
          onToggle={toggle}
        />
        <ComputerAction computer={computer} groupBy={groupBy} />
      </div>
      {refused ? <Note icon={Ban}>Sessions stay on the computer that runs them.</Note> : null}
      {look.query !== "" && computer.read === "offline" ? (
        <Note icon={WifiOff}>Matches from the rows seen before it went offline.</Note>
      ) : null}
      {open ? (
        <div className="pl-2.5">
          <ComputerBody computer={computer} groupBy={groupBy} />
        </div>
      ) : null}
    </section>
  );
}
