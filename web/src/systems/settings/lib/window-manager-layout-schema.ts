import { z } from "zod";

import type {
  WindowManagerLayoutApplyResult,
  WindowManagerLayoutChanges,
  WindowManagerLayoutDiagnostic,
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutPreview,
  WindowManagerLayoutProfile,
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutValidation,
  WindowManagerLayoutWindow,
  WindowManagerNormalizedRect,
} from "./window-manager-layout-types";
import type { WindowManagerConfig } from "@/systems/os";

const identifierSchema = z.string().trim().min(1);
const safeRevisionSchema = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);

const normalizedRectSchema = z
  .strictObject({
    x: z.number().finite(),
    y: z.number().finite(),
    width: z.number().finite().positive(),
    height: z.number().finite().positive(),
  })
  .transform(
    (rect): WindowManagerNormalizedRect => ({
      x: rect.x,
      y: rect.y,
      w: rect.width,
      h: rect.height,
    })
  );

const layoutNodeSchema: z.ZodType<WindowManagerLayoutNode> = z.lazy(() =>
  z.union([
    z
      .strictObject({
        id: identifierSchema,
        kind: z.literal("leaf"),
        window_id: identifierSchema,
      })
      .transform(node => ({ id: node.id, kind: node.kind, windowId: node.window_id })),
    z
      .strictObject({
        id: identifierSchema,
        kind: z.literal("stack"),
        window_ids: z.array(identifierSchema).min(1),
        active_id: identifierSchema,
      })
      .refine(node => node.window_ids.includes(node.active_id), {
        message: "active_id must name one stack window",
      })
      .transform(node => ({
        id: node.id,
        kind: node.kind,
        windowIds: node.window_ids,
        activeId: node.active_id,
      })),
    z
      .strictObject({
        id: identifierSchema,
        kind: z.literal("split"),
        axis: z.enum(["horizontal", "vertical"]),
        children: z.array(layoutNodeSchema).min(2),
        weights: z.array(z.number().finite().positive()).min(2),
      })
      .refine(node => node.children.length === node.weights.length, {
        message: "split children and weights must have equal lengths",
      })
      .transform(node => ({
        id: node.id,
        kind: node.kind,
        axis: node.axis,
        children: node.children,
        weights: node.weights,
      })),
  ])
);

const desktopSchema = z
  .strictObject({
    id: identifierSchema,
    name: z.string().trim().min(1),
    order: z.number().int().nonnegative(),
    purpose: z.enum(["standard", "focus"]),
    focus_owner: identifierSchema.optional(),
    groups: z.array(
      z
        .strictObject({
          id: identifierSchema,
          frame: normalizedRectSchema,
          root: layoutNodeSchema,
        })
        .transform(group => ({ id: group.id, frame: group.frame, root: group.root }))
    ),
    floating: z.array(identifierSchema),
  })
  .transform(desktop => ({
    id: desktop.id,
    name: desktop.name,
    order: desktop.order,
    purpose: desktop.purpose,
    focusOwner: desktop.focus_owner ?? null,
    groups: desktop.groups,
    floating: desktop.floating,
  }));

const layoutWindowSchema = z
  .strictObject({
    id: identifierSchema,
    app: identifierSchema,
    instance_key: z.string().optional(),
    route: z.strictObject({
      pathname: z.string().trim().startsWith("/"),
      search: z.record(z.string(), z.unknown()),
    }),
    placement: z.enum(["tiled", "stacked", "floating"]),
    desktop_id: identifierSchema,
    floating_rect: normalizedRectSchema,
    minimized: z.boolean(),
    return_anchor: z.unknown().optional(),
  })
  .transform(
    (window): WindowManagerLayoutWindow => ({
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

export const windowManagerLayoutDocumentSchema = z
  .strictObject({
    version: z.number().int().positive(),
    workspace_id: z.string(),
    desktops: z.array(desktopSchema).min(1),
    windows: z.record(z.string(), layoutWindowSchema),
    overrides: z.record(z.string(), z.unknown()),
  })
  .transform(
    (document): WindowManagerLayoutDocument => ({
      version: document.version,
      workspaceId: document.workspace_id,
      desktops: document.desktops,
      windows: document.windows,
      overrides: document.overrides,
    })
  );

const diagnosticSchema = z
  .strictObject({
    code: z.string(),
    path: z.string().optional(),
    message: z.string(),
  })
  .transform(
    (diagnostic): WindowManagerLayoutDiagnostic => ({
      code: diagnostic.code,
      path: diagnostic.path ?? null,
      message: diagnostic.message,
    })
  );

const changesSchema = z
  .object({
    desktop_ids: z.array(identifierSchema).optional(),
    window_ids: z.array(identifierSchema).optional(),
    group_ids: z.array(identifierSchema).optional(),
    node_ids: z.array(identifierSchema).optional(),
    client_ids: z.array(identifierSchema).optional(),
  })
  .transform(
    (changes): WindowManagerLayoutChanges => ({
      desktopIds: changes.desktop_ids ?? [],
      windowIds: changes.window_ids ?? [],
      groupIds: changes.group_ids ?? [],
      nodeIds: changes.node_ids ?? [],
      clientIds: changes.client_ids ?? [],
    })
  );

const profileSchema = z
  .strictObject({
    version: z.literal(1),
    id: identifierSchema,
    display_name: z.string().trim().min(1),
    aspect_variant: z.enum(["any", "portrait", "landscape"]),
    participant_slots: z.array(identifierSchema).optional(),
    overflow_policy: z.enum(["stack", "reject"]),
    document: windowManagerLayoutDocumentSchema,
  })
  .transform(
    (profile): WindowManagerLayoutProfile => ({
      version: profile.version,
      id: profile.id,
      displayName: profile.display_name,
      aspectVariant: profile.aspect_variant,
      participantSlots: profile.participant_slots ?? [],
      overflowPolicy: profile.overflow_policy,
      document: profile.document,
    })
  );

const resourceRecordSchema = z
  .strictObject({
    kind: z.literal("window_layout"),
    id: identifierSchema,
    version: z.number().int().positive(),
    scope: z.strictObject({
      kind: z.enum(["global", "workspace"]),
      id: z.string().optional(),
    }),
    owner: z.unknown(),
    source: z.unknown(),
    spec: profileSchema,
    created_at: z.string(),
    updated_at: z.string(),
  })
  .transform(
    (record): WindowManagerLayoutResourceRecord => ({
      kind: record.kind,
      id: record.id,
      version: record.version,
      scope: { kind: record.scope.kind, id: record.scope.id ?? "" },
      spec: record.spec,
      createdAt: record.created_at,
      updatedAt: record.updated_at,
    })
  );

export function parseWindowManagerLayoutDocument(value: unknown): WindowManagerLayoutDocument {
  return windowManagerLayoutDocumentSchema.parse(value);
}

export function parseWindowManagerLayoutValidation(value: unknown): WindowManagerLayoutValidation {
  return z
    .strictObject({
      workspace_id: identifierSchema,
      valid: z.boolean(),
      diagnostics: z.array(diagnosticSchema).optional(),
    })
    .transform(validation => ({
      workspaceId: validation.workspace_id,
      valid: validation.valid,
      diagnostics: validation.diagnostics ?? [],
    }))
    .parse(value);
}

export function parseWindowManagerLayoutPreview(value: unknown): WindowManagerLayoutPreview {
  return z
    .object({
      snapshot: z.object({ revision: safeRevisionSchema }),
      changed: z.boolean(),
      changes: changesSchema,
      diagnostics: z.array(diagnosticSchema).optional(),
    })
    .transform(preview => ({
      revision: preview.snapshot.revision,
      changed: preview.changed,
      changes: preview.changes,
      diagnostics: preview.diagnostics ?? [],
    }))
    .parse(value);
}

export function parseWindowManagerLayoutApply(value: unknown): WindowManagerLayoutApplyResult {
  return z
    .object({
      snapshot: z.object({ revision: safeRevisionSchema }),
      applied: z.boolean(),
      diagnostics: z.array(diagnosticSchema).optional(),
    })
    .transform(result => ({
      revision: result.snapshot.revision,
      applied: result.applied,
      diagnostics: result.diagnostics ?? [],
    }))
    .parse(value);
}

export function parseWindowManagerLayoutResources(
  value: unknown
): WindowManagerLayoutResourceRecord[] {
  return z.strictObject({ records: z.array(resourceRecordSchema) }).parse(value).records;
}

export function parseWindowManagerLayoutResource(
  value: unknown
): WindowManagerLayoutResourceRecord {
  return z.strictObject({ record: resourceRecordSchema }).parse(value).record;
}

function rectToWire(rect: WindowManagerNormalizedRect) {
  return { x: rect.x, y: rect.y, width: rect.w, height: rect.h };
}

function nodeToWire(node: WindowManagerLayoutNode): Record<string, unknown> {
  if (node.kind === "leaf") {
    return { id: node.id, kind: node.kind, window_id: node.windowId };
  }
  if (node.kind === "stack") {
    return {
      id: node.id,
      kind: node.kind,
      window_ids: node.windowIds,
      active_id: node.activeId,
    };
  }
  return {
    id: node.id,
    kind: node.kind,
    axis: node.axis,
    children: node.children.map(nodeToWire),
    weights: node.weights,
  };
}

export function windowManagerLayoutDocumentToWire(
  document: WindowManagerLayoutDocument
): Record<string, unknown> {
  return {
    version: document.version,
    workspace_id: document.workspaceId,
    desktops: document.desktops.map(desktop => ({
      id: desktop.id,
      name: desktop.name,
      order: desktop.order,
      purpose: desktop.purpose,
      ...(desktop.focusOwner ? { focus_owner: desktop.focusOwner } : {}),
      groups: desktop.groups.map(group => ({
        id: group.id,
        frame: rectToWire(group.frame),
        root: nodeToWire(group.root),
      })),
      floating: desktop.floating,
    })),
    windows: Object.fromEntries(
      Object.entries(document.windows).map(([id, window]) => [
        id,
        {
          id: window.id,
          app: window.app,
          ...(window.instanceKey ? { instance_key: window.instanceKey } : {}),
          route: window.route,
          placement: window.placement,
          desktop_id: window.desktopId,
          floating_rect: rectToWire(window.floatingRect),
          minimized: window.minimized,
          ...(window.returnAnchor ? { return_anchor: window.returnAnchor } : {}),
        },
      ])
    ),
    overrides: document.overrides,
  };
}

export function windowManagerProfileToWire(profile: WindowManagerLayoutProfile) {
  return {
    version: profile.version,
    id: profile.id,
    display_name: profile.displayName,
    aspect_variant: profile.aspectVariant,
    participant_slots: profile.participantSlots,
    overflow_policy: profile.overflowPolicy,
    document: windowManagerLayoutDocumentToWire(profile.document),
  };
}

export function windowManagerSettingsConfigToWire(config: WindowManagerConfig) {
  return {
    new_window_policy: config.newWindowPolicy,
    small_viewport_policy: config.smallViewportPolicy,
    focus_policy: config.focusPolicy,
    focus_wrap: config.focusWrap,
    focus_follows_pointer: config.focusFollowsPointer,
    raise_on_focus: config.raiseOnFocus,
    drag_away_policy: config.dragAwayPolicy,
    group_move_modifier: config.groupMoveModifier,
    swap_modifier: config.swapModifier,
    history_limit: config.historyLimit,
    desktop_transition: config.desktopTransition,
    gaps: config.gaps,
    snap: {
      edge_band: config.snap.edgeBand,
      corner_reach: config.snap.cornerReach,
      exit_slack: config.snap.exitSlack,
      repeat_ratios: config.snap.repeatRatios,
    },
    bindings: {
      top_center: config.bindings.topCenter,
      bottom_center: config.bindings.bottomCenter,
    },
    shortcuts: config.shortcuts,
  };
}
