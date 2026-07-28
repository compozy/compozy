import { z } from "zod";

import { windowManagerWorkspaceConfigSchema } from "./window-manager-config-schema";
import type {
  LayoutDesktop,
  LayoutNode,
  NormalizedRect,
  WindowManagerActor,
  WindowManagerChangeSet,
  WindowManagerClientView,
  WindowManagerCommandId,
  WindowManagerCommandResult,
  WindowManagerConflictPayload,
  WindowManagerDiagnosticPayload,
  WindowManagerErrorPayload,
  WindowManagerEvent,
  WindowManagerHistoryEntry,
  WindowManagerReturnAnchor,
  WindowManagerSnapshot,
  WindowManagerStateDocument,
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

const desktopSchema = z
  .strictObject({
    id: identifierSchema,
    name: z.string().trim().min(1),
    order: z.number().int().nonnegative(),
    purpose: z.enum(["standard", "focus"]),
    focus_owner: identifierSchema.optional(),
    groups: z.array(groupSchema),
    floating: z.array(identifierSchema),
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
      placement: window.placement,
      desktopId: window.desktop_id,
      floatingRect: window.floating_rect,
      minimized: window.minimized,
      returnAnchor: window.return_anchor ?? null,
    })
  );

const windowsSchema = z.record(z.string(), windowSchema);

const stateDocumentSchema = z
  .strictObject({
    desktops: z.array(desktopSchema),
    windows: windowsSchema,
    overrides: windowManagerWorkspaceConfigSchema,
  })
  .transform(
    (state): WindowManagerStateDocument => ({
      desktops: state.desktops,
      windows: state.windows,
      overrides: state.overrides,
    })
  );

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
  "layout.arrange",
  "layout.resize",
  "layout.balance",
  "layout.undo",
  "layout.redo",
  "layout.replace",
]);

const historyEntrySchema = z
  .strictObject({
    command_id: commandIdSchema,
    before: stateDocumentSchema,
    after: stateDocumentSchema,
    actor: actorSchema,
    origin: z.string().optional(),
    created_at: timestampSchema,
  })
  .transform(
    (entry): WindowManagerHistoryEntry => ({
      commandId: entry.command_id as WindowManagerCommandId,
      before: entry.before,
      after: entry.after,
      actor: entry.actor,
      origin: entry.origin ?? "",
      createdAt: entry.created_at,
    })
  );

export const windowManagerSnapshotSchema = z
  .strictObject({
    version: z.number().int().positive(),
    workspace_id: identifierSchema,
    revision: safeRevisionSchema,
    desktops: z.array(desktopSchema).min(1),
    windows: windowsSchema,
    history: z.strictObject({
      undo: z.array(historyEntrySchema),
      redo: z.array(historyEntrySchema),
    }),
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
      history: snapshot.history,
      overrides: snapshot.overrides,
      updatedAt: snapshot.updated_at,
    })
  );

export const windowManagerClientViewSchema = z
  .strictObject({
    workspace_id: identifierSchema,
    client_id: identifierSchema,
    presentation_revision: safeRevisionSchema,
    active_desktop_id: identifierSchema,
    focused_window_id: identifierSchema.optional(),
    focus_order: z.array(identifierSchema),
    connected_at: timestampSchema,
  })
  .transform(
    (client): WindowManagerClientView => ({
      workspaceId: client.workspace_id,
      clientId: client.client_id,
      presentationRevision: client.presentation_revision,
      activeDesktopId: client.active_desktop_id,
      focusedWindowId: client.focused_window_id ?? null,
      focusOrder: client.focus_order,
      connectedAt: client.connected_at,
    })
  );

const changeSetSchema = z
  .strictObject({
    desktop_ids: z.array(identifierSchema).optional(),
    window_ids: z.array(identifierSchema).optional(),
    group_ids: z.array(identifierSchema).optional(),
    node_ids: z.array(identifierSchema).optional(),
    client_ids: z.array(identifierSchema).optional(),
  })
  .transform(
    (changes): WindowManagerChangeSet => ({
      desktopIds: changes.desktop_ids ?? [],
      windowIds: changes.window_ids ?? [],
      groupIds: changes.group_ids ?? [],
      nodeIds: changes.node_ids ?? [],
      clientIds: changes.client_ids ?? [],
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

export function parseWindowManagerClientView(value: unknown): WindowManagerClientView {
  return windowManagerClientViewSchema.parse(value);
}

export function parseWindowManagerCommandResult(value: unknown): WindowManagerCommandResult {
  return windowManagerCommandResultSchema.parse(value);
}

export function parseWindowManagerError(value: unknown): WindowManagerErrorPayload {
  return windowManagerErrorSchema.parse(value);
}
