/**
 * The data an open session menu reads: the session's full linked work (`useSessionLinkedWork`)
 * and, for a host without a folder layout of its own, the session's folder layout. The menu mounts
 * this only while open, so a closed row or card reads nothing. Each read keeps one wrapper and
 * gates its query instead, so a lead or placement that changes while the menu is open never
 * remounts the rows (an open submenu stays open, focus stays put).
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

/** The `@user` selector for a User-scope session, its project id otherwise. */
function scopeSelector(session: SessionPayload): string {
  return workspaceSelectorForScope(ownerDataScope(session.workspace_id));
}

/** Only a session the catalog says has linked work reads its full list. */
function LinkedWorkSource({
  session,
  children,
}: {
  session: SessionPayload;
  children: Render<SessionMenuSourceData["linkedWork"]>;
}) {
  const lead = !!session.linked_work?.lead;
  const model = useSessionLinkedWork({
    session,
    workspaceId: scopeSelector(session),
    // A menu reads the list once; only a visible list keeps rereading it.
    enabled: false,
    read: lead,
  });
  return children(
    lead
      ? {
          items: model.items,
          total: model.count.total,
          refreshing: model.refreshing,
          onRefresh: model.refresh,
          onUnlink: model.unlink,
        }
      : null
  );
}

/** Folders hold a scope's top-level sessions, User scope (`@user`) included. */
function FolderSource({
  session,
  children,
}: {
  session: SessionPayload;
  children: Render<SessionMenuSourceData["layout"]>;
}) {
  const placed = sessionHasOwnPlacement(session) && sessionTreeParentId(session) === "";
  const { layout } = useSessionFolders({
    workspaceId: scopeSelector(session),
    profile: session.profile_name,
    enabled: placed,
  });
  if (!placed || !layout) return children(undefined);
  return children({
    folders: layout.folders,
    folderId: sessionFolderId(session),
    onMove: folderId => layout.moveSession(session.id, folderId),
  });
}
