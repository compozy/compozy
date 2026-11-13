import { z } from "zod";

import { windowManagerWorkspaceConfigSchema } from "./window-manager-config-schema";
import { globalShortcutRegistrationSchema } from "./window-manager-global-shortcut-schema";
import type {
  LayoutDesktop,
  LayoutNode,
  LayoutTab,
  WindowManagerActor,
  WindowManagerAttachedClientView,
  WindowManagerChangeSet,
  WindowManagerCommandResult,
  WindowManagerConflictPayload,
  WindowManagerDeprecation,
  WindowManagerDiagnosticPayload,
  WindowManagerErrorPayload,
  WindowManagerEvent,
  WindowManagerMigrationStatus,
  WindowManagerRegisteredClientView,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "./window-manager-types";

const safeRevisionSchema = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const identifierSchema = z.string().trim().min(1);
const timestampSchema = z.iso.datetime({ offset: true });

const actorSchema = z
  .strictObject({ kind: z.string().default(""), id: z.string().default("") })
  .transform((actor): WindowManagerActor => actor);

const diagnosticSchema = z
  .strictObject({
    code: z.string(),
    path: z.string().optional(),
    message: z.string(),
  })
  .transform((diagnostic): WindowManagerDiagnosticPayload => ({
    code: diagnostic.code,
    path: diagnostic.path ?? null,
    message: diagnostic.message,
  }));

const conflictSchema = z
  .strictObject({
    code: z.string(),
    entity_id: z.string().optional(),
    current_id: z.string().optional(),
  })
  .transform((conflict): WindowManagerConflictPayload => ({
    code: conflict.code,
    entityId: conflict.entity_id ?? null,
    currentId: conflict.current_id ?? null,
  }));

const leafNodeSchema = z
  .strictObject({
    id: identifierSchema,
    kind: z.literal("leaf"),
    window_id: identifierSchema,
  })
  .transform((node): LayoutNode => ({
    id: node.id,
    kind: node.kind,
    windowId: node.window_id,
  }));

const layoutNodeSchema: z.ZodType<LayoutNode> = z.lazy(() =>
  z.union([
    leafNodeSchema,
    z
      .strictObject({
        id: identifierSchema,
        kind: z.literal("split"),
        axis: z.enum(["horizontal", "vertical"]),
        children: z.array(layoutNodeSchema).min(2),
        weights: z.array(z.number().finite().positive()).min(2),
      })
      .refine(node => node.children.length === node.weights.length, {
        message: "split children and weights must have the same length",
      })
      .transform((node): LayoutNode => ({
        id: node.id,
        kind: node.kind,
        axis: node.axis,
        children: node.children,
        weights: node.weights,
      })),
  ])
);

const tabSchema = z
  .strictObject({
    id: identifierSchema,
    pinned: z.boolean(),
    root: layoutNodeSchema,
  })
  .transform((tab): LayoutTab => ({ id: tab.id, pinned: tab.pinned, root: tab.root }));

const desktopSchema = z
  .strictObject({
    id: identifierSchema,
    name: z.string().trim().min(1),
    order: z.number().int().nonnegative(),
    tabs: z.array(tabSchema).min(1),
    active_tab_id: identifierSchema,
  })
  .transform((desktop): LayoutDesktop => ({
    id: desktop.id,
    name: desktop.name,
    order: desktop.order,
    tabs: desktop.tabs,
    activeTabId: desktop.active_tab_id,
  }));

const routeSchema = z
  .strictObject({
    pathname: z.string().trim().startsWith("/"),
    search: z.record(z.string(), z.unknown()),
  })
  .transform(route => ({ pathname: route.pathname, search: route.search }));

const windowSchema = z
  .strictObject({
    id: identifierSchema,
    app: identifierSchema,
    instance_key: z.string().optional(),
    route: routeSchema,
    nav_stack: z.array(routeSchema),
    desktop_id: identifierSchema,
    tab_id: identifierSchema,
  })
  .transform((window): WindowManagerWindow => ({
    id: window.id,
    app: window.app,
    instanceKey: window.instance_key ?? null,
    route: window.route,
    navStack: window.nav_stack,
    desktopId: window.desktop_id,
    tabId: window.tab_id,
  }));

const windowsSchema = z.record(z.string(), windowSchema);

const commandIdSchema = z.enum([
  "desktop.create",
  "desktop.update",
  "desktop.reorder",
  "desktop.switch",
  "desktop.delete",
  "window.open",
  "window.navigate",
  "window.close",
  "window.focus",
  "window.move",
  "window.resize",
  "window.swap",
  "window.toggle_floating",
  "window.zoom",
  "window.stack.group",
  "window.stack.reorder",
  "window.stack.set_active",
  "window.pin",
  "window.reopen",
  "window.tab.reorder",
  "window.tab.activate",
  "layout.arrange",
  "layout.resize",
  "layout.frame_resize",
  "layout.balance",
  "layout.undo",
  "layout.redo",
  "layout.replace",
]);

const migrationSummarySchema = z.object({
  desktops: z.number().int().nonnegative(),
  tabs: z.number().int().nonnegative(),
  windows: z.number().int().nonnegative(),
  floating_to_tabs: z.number().int().nonnegative(),
  minimized_to_tabs: z.number().int().nonnegative(),
  islands_merged: z.number().int().nonnegative(),
  pins_dropped: z.number().int().nonnegative(),
  zoom_desktops_merged: z.number().int().nonnegative(),
});

const migrationStatusSchema = z
  .object({
    state: z.string(),
    from_version: z.number().int().nonnegative().optional(),
    at: timestampSchema.optional(),
    acknowledged: z.boolean(),
    summary: migrationSummarySchema.nullish(),
    error: diagnosticSchema.nullish(),
    failure_revision: safeRevisionSchema.optional(),
  })
  .transform(
    (status): WindowManagerMigrationStatus => ({
      state: status.state,
      fromVersion: status.from_version ?? null,
      at: status.at ?? null,
      acknowledged: status.acknowledged,
      summary: status.summary
        ? {
            desktops: status.summary.desktops,
            tabs: status.summary.tabs,
            windows: status.summary.windows,
            floatingToTabs: status.summary.floating_to_tabs,
            minimizedToTabs: status.summary.minimized_to_tabs,
            islandsMerged: status.summary.islands_merged,
            pinsDropped: status.summary.pins_dropped,
            zoomDesktopsMerged: status.summary.zoom_desktops_merged,
          }
        : null,
      error: status.error ?? null,
      failureRevision: status.failure_revision ?? null,
    })
  );

const deprecationSchema = z
  .object({
    code: z.string(),
    surface: z.string(),
    replacement: z.string().optional(),
    removal_release: z.string(),
    message: z.string(),
  })
  .transform(
    (deprecation): WindowManagerDeprecation => ({
      code: deprecation.code,
      surface: deprecation.surface,
      replacement: deprecation.replacement ?? null,
      removalRelease: deprecation.removal_release,
      message: deprecation.message,
    })
  );

export const windowManagerSnapshotSchema = z
  .strictObject({
    version: z.literal(6),
    workspace_id: identifierSchema,
    revision: safeRevisionSchema,
    desktops: z.array(desktopSchema).min(1),
    windows: windowsSchema,
    closed_entry_count: z.number().int().nonnegative(),
    overrides: windowManagerWorkspaceConfigSchema,
    migration: migrationStatusSchema.nullish(),
    updated_at: timestampSchema,
  })
  .transform((snapshot): WindowManagerSnapshot => ({
    version: snapshot.version,
    workspaceId: snapshot.workspace_id,
    revision: snapshot.revision,
    desktops: snapshot.desktops,
    windows: snapshot.windows,
    closedEntryCount: snapshot.closed_entry_count,
    overrides: snapshot.overrides,
    migration: snapshot.migration ?? null,
    updatedAt: snapshot.updated_at,
  }));

export const windowManagerClientViewSchema = z
  .strictObject({
    workspace_id: identifierSchema,
    client_id: identifierSchema,
    kind: z.enum(["shell", "browser"]),
    presentation_revision: safeRevisionSchema,
    context_revision: safeRevisionSchema,
    active_desktop_id: identifierSchema,
    focused_window_id: identifierSchema.optional(),
    focus_order: z.array(identifierSchema),
    active_tab: z.record(z.string(), identifierSchema),
    // Retired fields the daemon still sends through v0.7.0: always `{}`, `false`, and a
    // mirror of `window_in_split`. Accepted and ignored.
    stack_active: z.record(z.string(), identifierSchema).optional(),
    palette_context: z.strictObject({
      window_focused: z.boolean(),
      window_in_split: z.boolean(),
      window_floating: z.boolean().optional(),
      window_stacked: z.boolean().optional(),
      desktop_window_count: z.number().int().nonnegative(),
      scope_global: z.boolean(),
      shell_desktop: z.boolean(),
      focused_session_state: z.string().optional(),
      workspace_trusted: z.boolean(),
      destination_intent: routeSchema.optional(),
    }),
    connected_at: timestampSchema,
    attachment_token: identifierSchema.optional(),
    global_shortcuts: z.array(globalShortcutRegistrationSchema),
  })
  .transform((client): WindowManagerAttachedClientView => ({
    workspaceId: client.workspace_id,
    clientId: client.client_id,
    kind: client.kind,
    presentationRevision: client.presentation_revision,
    contextRevision: client.context_revision,
    activeDesktopId: client.active_desktop_id,
    focusedWindowId: client.focused_window_id ?? null,
    focusOrder: client.focus_order,
    activeTab: client.active_tab,
    paletteContext: {
      windowFocused: client.palette_context.window_focused,
      windowInSplit: client.palette_context.window_in_split,
      desktopWindowCount: client.palette_context.desktop_window_count,
      scopeGlobal: client.palette_context.scope_global,
      shellDesktop: client.palette_context.shell_desktop,
      focusedSessionState: client.palette_context.focused_session_state ?? null,
      workspaceTrusted: client.palette_context.workspace_trusted,
      destinationIntent: client.palette_context.destination_intent ?? null,
    },
    connectedAt: client.connected_at,
    globalShortcuts: client.global_shortcuts.map(registration => ({
      commandId: registration.command_id,
      intendedChord: registration.intended_chord,
      activeChord: registration.active_chord ?? null,
      status: registration.status,
      reason: registration.reason ?? null,
      settingsUrl: registration.settings_url ?? null,
    })),
  }));

const changeSetSchema = z
  .strictObject({
    desktop_ids: z.array(identifierSchema).optional(),
    window_ids: z.array(identifierSchema).optional(),
    tab_ids: z.array(identifierSchema).optional(),
    node_ids: z.array(identifierSchema).optional(),
    client_ids: z.array(identifierSchema).optional(),
    tab_split: z.array(identifierSchema).optional(),
    tab_unsplit: z.array(identifierSchema).optional(),
    // Retired through v0.7.0 and always empty: accepted and ignored.
    group_ids: z.array(identifierSchema).optional(),
    stack_grouped: z.array(identifierSchema).optional(),
    stack_ungrouped: z.array(identifierSchema).optional(),
  })
  .transform((changes): WindowManagerChangeSet => ({
    desktopIds: changes.desktop_ids ?? [],
    windowIds: changes.window_ids ?? [],
    tabIds: changes.tab_ids ?? [],
    nodeIds: changes.node_ids ?? [],
    clientIds: changes.client_ids ?? [],
    tabSplit: changes.tab_split ?? [],
    tabUnsplit: changes.tab_unsplit ?? [],
  }));

export const windowManagerCommandResultSchema = z
  .strictObject({
    snapshot: windowManagerSnapshotSchema,
    applied: z.boolean(),
    changes: changeSetSchema,
    diagnostics: z.array(diagnosticSchema).optional(),
    deprecations: z.array(deprecationSchema).optional(),
    client: windowManagerClientViewSchema.optional(),
    rebased_from: safeRevisionSchema.optional(),
  })
  .transform((result): WindowManagerCommandResult => ({
    snapshot: result.snapshot,
    applied: result.applied,
    changes: result.changes,
    diagnostics: result.diagnostics ?? [],
    deprecations: result.deprecations ?? [],
    client: result.client ?? null,
    rebasedFrom: result.rebased_from ?? null,
  }));

export const windowManagerEventSchema = z
  .strictObject({
    workspace_id: identifierSchema,
    revision: safeRevisionSchema,
    command_id: commandIdSchema,
    changes: changeSetSchema,
    actor: actorSchema,
    origin: z.string().optional(),
    occurred_at: timestampSchema,
  })
  .transform((event): WindowManagerEvent => ({
    workspaceId: event.workspace_id,
    revision: event.revision,
    commandId: event.command_id,
    changes: event.changes,
    actor: event.actor,
    origin: event.origin ?? "",
    occurredAt: event.occurred_at,
  }));

export const windowManagerErrorSchema = z
  .strictObject({
    error: z.string(),
    code: z.string(),
    workspace_id: z.string(),
    current_revision: safeRevisionSchema.optional(),
    conflicts: z.array(conflictSchema).optional(),
    diagnostics: z.array(diagnosticSchema).optional(),
  })
  .transform((error): WindowManagerErrorPayload => ({
    error: error.error,
    code: error.code,
    workspaceId: error.workspace_id,
    currentRevision: error.current_revision ?? null,
    conflicts: error.conflicts ?? [],
    diagnostics: error.diagnostics ?? [],
  }));

export function parseWindowManagerSnapshot(value: unknown): WindowManagerSnapshot {
  return windowManagerSnapshotSchema.parse(value);
}

export function parseWindowManagerClientView(value: unknown): WindowManagerAttachedClientView {
  return windowManagerClientViewSchema.parse(value);
}

export function parseWindowManagerRegisteredClientView(
  value: unknown
): WindowManagerRegisteredClientView {
  const client = parseWindowManagerClientView(value);
  const parsedToken = z.object({ attachment_token: identifierSchema }).safeParse(value);
  if (!parsedToken.success) {
    throw new Error("Window-manager registration omitted attachment_token.");
  }
  return { ...client, attachmentToken: parsedToken.data.attachment_token };
}

export function parseWindowManagerCommandResult(value: unknown): WindowManagerCommandResult {
  return windowManagerCommandResultSchema.parse(value);
}

export function parseWindowManagerError(value: unknown): WindowManagerErrorPayload {
  return windowManagerErrorSchema.parse(value);
}
