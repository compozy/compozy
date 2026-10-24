import {
  GLOBAL_DESKTOP_WORKSPACE_ID,
  sameDataScope,
  USER_DESKTOP_WORKSPACE_ID,
  WINDOW_BINDING_SEARCH_KEY,
  windowBindingValue,
  type DataScope,
  type FocusedDataScope,
} from "@/systems/workspace";

import type { OsAppId, OsOpenTarget, OsWindowRoute } from "./os-types";
import type { RoutingCoordinator } from "./routing-coordinator";

/** One item to open: its app window, the scope it belongs to, and its route. */
export interface OpenTarget {
  app: OsAppId;
  instanceKey?: string;
  /** The item's own scope: its project, or User. */
  binding: FocusedDataScope;
  route: OsWindowRoute;
  placement?: OsOpenTarget["placement"];
  desktopId?: string;
  /** A session this launcher just created lands in an existing Session window (#825). */
  created?: boolean;
}

export interface OpenContext {
  coordinator: Pick<RoutingCoordinator, "userOpen" | "userOpenCreatedSession">;
  /** The menubar destination; `null` while unsettled. */
  dataScope: DataScope | null;
  /** The partition whose window manager accepts commands right now, if any. */
  commandPartition: string | null;
  /** Focus another project or User; the open waits for its desktop (project mode). */
  focusDestination: (scope: FocusedDataScope) => void;
  /** Hold the open until `commandPartition` reaches `partition` and the focus matches. */
  defer: (pending: PendingOpen) => void;
}

/** An open waiting for its partition's window manager to become live. */
export interface PendingOpen {
  partition: string;
  target: OsOpenTarget & { route: OsWindowRoute };
  created: boolean;
}

function partitionOf(scope: FocusedDataScope): string {
  return scope.kind === "user" ? USER_DESKTOP_WORKSPACE_ID : scope.workspaceId;
}

/** The route with the item's binding, so the window reads that item's scope in any partition. */
export function boundRoute(target: Pick<OpenTarget, "binding" | "route">): OsWindowRoute {
  return {
    ...target.route,
    search: {
      ...target.route.search,
      [WINDOW_BINDING_SEARCH_KEY]: windowBindingValue(target.binding),
    },
  };
}

function osTarget(target: OpenTarget, route: OsWindowRoute): PendingOpen["target"] {
  return {
    app: target.app,
    ...(target.instanceKey ? { instanceKey: target.instanceKey } : {}),
    route,
    ...(target.placement ? { placement: target.placement } : {}),
    ...(target.desktopId ? { desktopId: target.desktopId } : {}),
  };
}

/** Runs an open the window manager can accept now. */
export function runOpen(
  coordinator: OpenContext["coordinator"],
  pending: Pick<PendingOpen, "target" | "created">
): Promise<unknown> {
  const { instanceKey } = pending.target;
  return pending.created && instanceKey
    ? coordinator.userOpenCreatedSession({ ...pending.target, instanceKey })
    : coordinator.userOpen(pending.target);
}

/**
 * The one open path for items. Under All projects the item opens in place in
 * the `global` desktop, bound to its scope, and breadth never changes. With a
 * project or User focused, an item of that scope opens where it is; any other
 * item focuses its destination first (today's switch) and opens once that
 * desktop is live.
 */
export function openTarget(target: OpenTarget, ctx: OpenContext): Promise<void> {
  const scope = ctx.dataScope;
  const created = target.created === true;
  let partition: string;
  let route = target.route;
  if (scope?.kind === "all") {
    partition = GLOBAL_DESKTOP_WORKSPACE_ID;
    route = boundRoute(target);
  } else {
    partition = partitionOf(target.binding);
    if (scope === null || !sameDataScope(scope, target.binding)) {
      ctx.defer({ partition, target: osTarget(target, route), created });
      ctx.focusDestination(target.binding);
      return Promise.resolve();
    }
  }
  const pending = { partition, target: osTarget(target, route), created };
  if (ctx.commandPartition !== partition) {
    ctx.defer(pending);
    return Promise.resolve();
  }
  return runOpen(ctx.coordinator, pending).then(() => undefined);
}
