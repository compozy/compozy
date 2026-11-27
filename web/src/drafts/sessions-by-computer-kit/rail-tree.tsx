/**
 * The rows of a session list that knows its computers. A computer section holds one daemon's
 * list; inside it, project groups and folders work as they do today. A folder belongs to one
 * computer, so it is created and shown only inside that computer.
 */
import { use } from "react";

import { ComputerPlatformIcon } from "@/systems/computers/components/computer-platform-icon";
import { SessionListCreateMenu } from "@/systems/session/components/session-list/session-list-create-menu";
import { SessionListFolder } from "@/systems/session/components/session-list/session-list-folder";
import { SessionListGroupToggle } from "@/systems/session/components/session-list/session-list-group-toggle";
import { SessionListRowView } from "@/systems/session/components/session-list/session-list-row-view";
import { SessionRowActions } from "@/systems/session/components/session-row-actions";
import { SessionRowContextMenu } from "@/systems/session/components/session-row-context-menu";
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
import { useFold } from "./hooks/use-fold";
import { byProject, leadingOf, type RailComputer } from "./model";
import { RailLookContext } from "./rail-look";

export type { RailLook } from "./rail-look";

const actions = sessionLifecycleActionsFixture();

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
