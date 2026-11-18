import { use } from "react";

import { MenuTreeItems, type MenuTreeHost } from "@compozy/ui";

import { SessionMenuSourceContext } from "../contexts/session-menu-source-context-value";
import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";
import {
  useSessionRowMenuTree,
  type SessionRowMenuOptions,
} from "../hooks/use-session-row-menu-tree";
import type { SessionRowLayoutMenu } from "../lib/session-menu-tree";
import type { SessionPayload } from "../types";

export interface SessionRowMenuProps {
  session: SessionPayload;
  actions: SessionLifecycleActionHandlers;
  /** A list root row's folder moves and manual order. */
  layout?: SessionRowLayoutMenu;
  /** The host has no folder layout of its own (a board card): the menu reads the session's. */
  readFolders?: boolean;
  /** Opens this session; Link another… and Show all need its window. */
  onOpen?: () => void;
}

/**
 * One session's menu rows inside a context menu or dropdown. It mounts only while its menu is
 * open, and only then reads the session's full linked work (and, for a card, its folders).
 */
export function SessionRowMenuItems({
  host,
  readFolders = false,
  ...props
}: SessionRowMenuProps & { host: MenuTreeHost }) {
  const Source = use(SessionMenuSourceContext);
  const testIdPrefix = host === "context" ? "session-row-menu" : "session-row";
  if (!Source) return <SessionRowMenuTree {...props} host={host} testIdPrefix={testIdPrefix} />;
  return (
    // oxlint-disable-next-line react/static-components -- Source is the module-level SessionMenuSource injected by SessionDeriveHost, so its identity never changes between renders.
    <Source folders={readFolders && !props.layout} session={props.session}>
      {data => (
        <SessionRowMenuTree
          {...props}
          host={host}
          layout={props.layout ?? data.layout}
          linkedWork={data.linkedWork}
          testIdPrefix={testIdPrefix}
        />
      )}
    </Source>
  );
}

function SessionRowMenuTree({
  session,
  actions,
  host,
  ...options
}: Omit<SessionRowMenuProps, "readFolders"> & SessionRowMenuOptions & { host: MenuTreeHost }) {
  const nodes = useSessionRowMenuTree(session, actions, options);
  return <MenuTreeItems host={host} nodes={nodes} />;
}
