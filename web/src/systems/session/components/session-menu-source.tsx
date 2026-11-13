/**
 * The data an open session menu reads: the session's full linked work (`useSessionLinkedWork`)
 * and, for a host without a folder layout of its own, the session's folder layout. The menu mounts
 * this only while open, so a closed row or card reads nothing.
 */
import type { ReactNode } from "react";

import { ownerDataScope, workspaceSelectorForScope } from "@/systems/workspace";

import type {
  SessionMenuSourceData,
  SessionMenuSourceProps,
} from "../contexts/session-menu-source-context-value";
import { useSessionFolders } from "../hooks/use-session-folders";
import { useSessionLinkedWork } from "../hooks/use-session-linked-work";
import { sessionFolderId, sessionHasOwnPlacement } from "../lib/session-folders";
import { sessionTreeParentId } from "../lib/session-hierarchy";
import type { SessionPayload } from "../types";

type Render<T> = (value: T) => ReactNode;

export function SessionMenuSource({ session, folders, children }: SessionMenuSourceProps) {
  return (
    <LinkedWorkSource session={session}>
      {linkedWork =>
        folders ? (
          <FolderSource session={session}>
            {layout => children({ linkedWork, ...(layout ? { layout } : {}) })}
          </FolderSource>
        ) : (
          children({ linkedWork })
        )
      }
    </LinkedWorkSource>
  );
}

type LinkedWork = SessionMenuSourceData["linkedWork"];

/** Only a session the catalog says has linked work reads its full list. */
function LinkedWorkSource({
  session,
  children,
}: {
  session: SessionPayload;
  children: Render<LinkedWork>;
}) {
  if (!session.linked_work?.lead) return children(null);
  return <LoadedLinkedWork session={session}>{children}</LoadedLinkedWork>;
}

function LoadedLinkedWork({
  session,
  children,
}: {
  session: SessionPayload;
  children: Render<LinkedWork>;
}) {
  const model = useSessionLinkedWork({
    session,
    workspaceId: workspaceSelectorForScope(ownerDataScope(session.workspace_id)),
    // A menu reads the list once; only a visible list keeps rereading it.
    enabled: false,
  });
  return children({
    items: model.items,
    total: model.count.total,
    refreshing: model.refreshing,
    onRefresh: model.refresh,
    onUnlink: model.unlink,
  });
}

type Layout = SessionMenuSourceData["layout"];

/** Folders hold top-level sessions of a project; User-scope sessions have none. */
function FolderSource({
  session,
  children,
}: {
  session: SessionPayload;
  children: Render<Layout>;
}) {
  const placed = sessionHasOwnPlacement(session) && sessionTreeParentId(session) === "";
  if (!placed || !session.workspace_id) return children(undefined);
  return (
    <LoadedFolders session={session} workspaceId={session.workspace_id}>
      {children}
    </LoadedFolders>
  );
}

function LoadedFolders({
  session,
  workspaceId,
  children,
}: {
  session: SessionPayload;
  workspaceId: string;
  children: Render<Layout>;
}) {
  const { layout } = useSessionFolders({
    workspaceId,
    profile: session.profile_name,
    enabled: true,
  });
  if (!layout) return children(undefined);
  return children({
    folders: layout.folders,
    folderId: sessionFolderId(session),
    onMove: folderId => layout.moveSession(session.id, folderId),
  });
}
