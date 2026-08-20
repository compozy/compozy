import { z } from "zod";

import { windowManagerWorkspaceConfigSchema } from "./window-manager-config-schema";
import { globalShortcutRegistrationSchema } from "./window-manager-global-shortcut-schema";
import type {
  LayoutDesktop,
  LayoutNode,
  NormalizedRect,
  WindowManagerActor,
  WindowManagerAttachedClientView,
  WindowManagerChangeSet,
  WindowManagerCommandResult,
  WindowManagerConflictPayload,
  WindowManagerDiagnosticPayload,
  WindowManagerErrorPayload,
  WindowManagerEvent,
  WindowManagerRegisteredClientView,
  WindowManagerReturnAnchor,
  WindowManagerSnapshot,
  WindowManagerWindow,
} from "./window-manager-types";

const safeRevisionSchema = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);
const identifierSchema = z.string().trim().min(1);
const timestampSchema = z.iso.datetime({ offset: true });

const normalizedRectSchema = z
  .strictObject({
    x: z.number().finite(),
    y: z.number().finite(),
    width: z.number().finite().positive(),
    height: z.number().finite().positive(),
  })
  .transform((rect): NormalizedRect => ({ x: rect.x, y: rect.y, w: rect.width, h: rect.height }));

const actorSchema = z
  .strictObject({ kind: z.string().default(""), id: z.string().default("") })
  .transform((actor): WindowManagerActor => actor);

const diagnosticSchema = z
  .strictObject({
    code: z.string(),
    path: z.string().optional(),
    message: z.string(),
  })
  .transform(
    (diagnostic): WindowManagerDiagnosticPayload => ({
      code: diagnostic.code,
      path: diagnostic.path ?? null,
      message: diagnostic.message,
    })
  );

const conflictSchema = z
  .strictObject({
    code: z.string(),
    entity_id: z.string().optional(),
    current_id: z.string().optional(),
  })
  .transform(
    (conflict): WindowManagerConflictPayload => ({
      code: conflict.code,
      entityId: conflict.entity_id ?? null,
      currentId: conflict.current_id ?? null,
    })
  );

const leafNodeSchema = z
  .strictObject({
    id: identifierSchema,
    kind: z.literal("leaf"),
    window_id: identifierSchema,
  })
  .transform(
    (node): LayoutNode => ({
      id: node.id,
      kind: node.kind,
      windowId: node.window_id,
    })
  );

const stackNodeSchema = z
  .strictObject({
    id: identifierSchema,
    kind: z.literal("stack"),
    window_ids: z.array(identifierSchema).min(1),
    active_id: identifierSchema,
  })
  .transform(
    (node): LayoutNode => ({
      id: node.id,
      kind: node.kind,
      windowIds: node.window_ids,
      activeId: node.active_id,
    })
  );

const layoutNodeSchema: z.ZodType<LayoutNode> = z.lazy(() =>
  z.union([
    leafNodeSchema,
    stackNodeSchema,
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
      .transform(
        (node): LayoutNode => ({
          id: node.id,
          kind: node.kind,
          axis: node.axis,
          children: node.children,
          weights: node.weights,
        })
      ),
  ])
);

const groupSchema = z
  .strictObject({
    id: identifierSchema,
    frame: normalizedRectSchema,
    root: layoutNodeSchema,
  })
  .transform(group => ({ id: group.id, frame: group.frame, root: group.root }));

const floatingStackSchema = z
  .strictObject({
    id: identifierSchema,
    window_ids: z.array(identifierSchema).min(1),
    active_id: identifierSchema.optional(),
    rect: normalizedRectSchema,
    minimized: z.boolean(),
  })
  .refine(stack => stack.active_id === undefined || stack.window_ids.includes(stack.active_id), {
    message: "active_id must name one floating stack window",
  })
  .transform(stack => ({
    id: stack.id,
    windowIds: stack.window_ids,
    activeId: stack.active_id ?? null,
    rect: stack.rect,
    minimized: stack.minimized,
  }));

const desktopSchema = z
  .strictObject({
    id: identifierSchema,
    name: z.string().trim().min(1),
    order: z.number().int().nonnegative(),
    purpose: z.enum(["standard", "focus"]),
    focus_owner: identifierSchema.optional(),
    groups: z.array(groupSchema),
    floating: z.array(identifierSchema),
    floating_stacks: z.array(floatingStackSchema),
  })
  .transform(
    (desktop): LayoutDesktop => ({
      id: desktop.id,
      name: desktop.name,
      order: desktop.order,
      purpose: desktop.purpose,
      focusOwner: desktop.focus_owner ?? null,
      groups: desktop.groups,
      floating: desktop.floating,
      floatingStacks: desktop.floating_stacks,
    })
  );

const returnAnchorSchema = z
  .strictObject({
    desktop_id: identifierSchema,
    group_id: identifierSchema.optional(),
    parent_split_id: identifierSchema.optional(),
    child_index: z.number().int().nonnegative().optional(),
    weight: z.number().finite().positive().optional(),
    neighbor_ids: z.array(identifierSchema).optional(),
    source_revision: safeRevisionSchema,
    source_group: groupSchema.optional(),
  })
  .transform(
    (anchor): WindowManagerReturnAnchor => ({
      desktopId: anchor.desktop_id,
      groupId: anchor.group_id ?? null,
      parentSplitId: anchor.parent_split_id ?? null,
      childIndex: anchor.child_index ?? null,
      weight: anchor.weight ?? null,
      neighborIds: anchor.neighbor_ids ?? [],
      sourceRevision: anchor.source_revision,
      sourceGroup: anchor.source_group ?? null,
    })
  );

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
    pinned: z.boolean(),
    placement: z.enum(["tiled", "stacked", "floating"]),
    desktop_id: identifierSchema,
    floating_rect: normalizedRectSchema,
    minimized: z.boolean(),
    return_anchor: returnAnchorSchema.optional(),
  })
  .transform(
    (window): WindowManagerWindow => ({
      id: window.id,
      app: window.app,
      instanceKey: window.instance_key ?? null,
      route: window.route,
      navStack: window.nav_stack,
      pinned: window.pinned,
      placement: window.placement,
      desktopId: window.desktop_id,
      floatingRect: window.floating_rect,
      minimized: window.minimized,
      returnAnchor: window.return_anchor ?? null,
    })
  );

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
  "window.swap",
  "window.toggle_floating",
  "window.zoom",
  "window.stack.group",
  "window.stack.reorder",
  "window.stack.set_active",
  "window.pin",
  "window.reopen",
  "layout.arrange",
  "layout.resize",
  "layout.balance",
  "layout.undo",
  "layout.redo",
  "layout.replace",
]);

export const windowManagerSnapshotSchema = z
  .strictObject({
    version: z.literal(3),
    workspace_id: identifierSchema,
    revision: safeRevisionSchema,
    desktops: z.array(desktopSchema).min(1),
    windows: windowsSchema,
    closed_entry_count: z.number().int().nonnegative(),
    overrides: windowManagerWorkspaceConfigSchema,
    updated_at: timestampSchema,
  })
  .transform(
    (snapshot): WindowManagerSnapshot => ({
      version: snapshot.version,
      workspaceId: snapshot.workspace_id,
      revision: snapshot.revision,
      desktops: snapshot.desktops,
      windows: snapshot.windows,
      closedEntryCount: snapshot.closed_entry_count,
      overrides: snapshot.overrides,
      updatedAt: snapshot.updated_at,
    })
  );

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
    stack_active: z.record(z.string(), identifierSchema),
    palette_context: z.strictObject({
      window_focused: z.boolean(),
      window_floating: z.boolean(),
      window_stacked: z.boolean(),
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
  .transform(
    (client): WindowManagerAttachedClientView => ({
      workspaceId: client.workspace_id,
      clientId: client.client_id,
      kind: client.kind,
      presentationRevision: client.presentation_revision,
      contextRevision: client.context_revision,
      activeDesktopId: client.active_desktop_id,
      focusedWindowId: client.focused_window_id ?? null,
      focusOrder: client.focus_order,
      stackActive: client.stack_active,
      paletteContext: {
        windowFocused: client.palette_context.window_focused,
        windowFloating: client.palette_context.window_floating,
        windowStacked: client.palette_context.window_stacked,
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
    })
  );

const changeSetSchema = z
  .strictObject({
    desktop_ids: z.array(identifierSchema).optional(),
    window_ids: z.array(identifierSchema).optional(),
    group_ids: z.array(identifierSchema).optional(),
    node_ids: z.array(identifierSchema).optional(),
    client_ids: z.array(identifierSchema).optional(),
    stack_grouped: z.array(identifierSchema).optional(),
    stack_ungrouped: z.array(identifierSchema).optional(),
  })
  .transform(
    (changes): WindowManagerChangeSet => ({
      desktopIds: changes.desktop_ids ?? [],
      windowIds: changes.window_ids ?? [],
      groupIds: changes.group_ids ?? [],
      nodeIds: changes.node_ids ?? [],
      clientIds: changes.client_ids ?? [],
      stackGrouped: changes.stack_grouped ?? [],
      stackUngrouped: changes.stack_ungrouped ?? [],
    })
  );

export const windowManagerCommandResultSchema = z
  .strictObject({
    snapshot: windowManagerSnapshotSchema,
    applied: z.boolean(),
    changes: changeSetSchema,
    diagnostics: z.array(diagnosticSchema).optional(),
    client: windowManagerClientViewSchema.optional(),
    rebased_from: safeRevisionSchema.optional(),
  })
  .transform(
    (result): WindowManagerCommandResult => ({
      snapshot: result.snapshot,
      applied: result.applied,
      changes: result.changes,
      diagnostics: result.diagnostics ?? [],
      client: result.client ?? null,
      rebasedFrom: result.rebased_from ?? null,
    })
  );

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
  .transform(
    (event): WindowManagerEvent => ({
      workspaceId: event.workspace_id,
      revision: event.revision,
      commandId: event.command_id,
      changes: event.changes,
      actor: event.actor,
      origin: event.origin ?? "",
      occurredAt: event.occurred_at,
    })
  );

export const windowManagerErrorSchema = z
  .strictObject({
    error: z.string(),
    code: z.string(),
    workspace_id: z.string(),
    current_revision: safeRevisionSchema.optional(),
    conflicts: z.array(conflictSchema).optional(),
    diagnostics: z.array(diagnosticSchema).optional(),
  })
  .transform(
    (error): WindowManagerErrorPayload => ({
      error: error.error,
      code: error.code,
      workspaceId: error.workspace_id,
      currentRevision: error.current_revision ?? null,
      conflicts: error.conflicts ?? [],
      diagnostics: error.diagnostics ?? [],
    })
  );

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
