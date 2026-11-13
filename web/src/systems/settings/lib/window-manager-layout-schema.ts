import { z } from "zod";

import type {
  WindowManagerLayoutApplyResult,
  WindowManagerLayoutChanges,
  WindowManagerLayoutDesktop,
  WindowManagerLayoutDiagnostic,
  WindowManagerLayoutDocument,
  WindowManagerLayoutNode,
  WindowManagerLayoutPreview,
  WindowManagerLayoutProfile,
  WindowManagerLayoutProfileSource,
  WindowManagerLayoutResourceRecord,
  WindowManagerLayoutTab,
  WindowManagerLayoutValidation,
  WindowManagerLayoutWindow,
} from "./window-manager-layout-types";

/** Versions the daemon converts on every read (`internal/windowmanager/layout_decode.go`). */
const EARLIER_LAYOUT_VERSIONS: ReadonlySet<number> = new Set([4, 5]);

const identifierSchema = z.string().trim().min(1);
const safeRevisionSchema = z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER);

const routeSchema = z
  .strictObject({
    pathname: z.string().trim().startsWith("/"),
    search: z.record(z.string(), z.unknown()),
  })
  .transform(route => ({ pathname: route.pathname, search: route.search }));

const layoutNodeSchema: z.ZodType<WindowManagerLayoutNode> = z.lazy(() =>
  z.union([
    z
      .strictObject({
        id: identifierSchema,
        kind: z.literal("leaf"),
        window_id: identifierSchema,
      })
      .transform((node): WindowManagerLayoutNode => ({
        id: node.id,
        kind: node.kind,
        windowId: node.window_id,
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
      .transform((node): WindowManagerLayoutNode => ({
        id: node.id,
        kind: node.kind,
        axis: node.axis,
        children: node.children,
        weights: node.weights,
      })),
  ])
);

const tabSchema = z
  .strictObject({ id: identifierSchema, pinned: z.boolean(), root: layoutNodeSchema })
  .transform((tab): WindowManagerLayoutTab => ({ id: tab.id, pinned: tab.pinned, root: tab.root }));

const desktopSchema = z
  .strictObject({
    id: identifierSchema,
    name: z.string().trim().min(1),
    order: z.number().int().nonnegative(),
    active_tab_id: identifierSchema,
    tabs: z.array(tabSchema).min(1),
  })
  .transform((desktop): WindowManagerLayoutDesktop => ({
    id: desktop.id,
    name: desktop.name,
    order: desktop.order,
    activeTabId: desktop.active_tab_id,
    tabs: desktop.tabs,
  }));

const layoutWindowSchema = z
  .strictObject({
    id: identifierSchema,
    app: identifierSchema,
    instance_key: z.string().nullish(),
    route: routeSchema,
    nav_stack: z.array(routeSchema).max(200),
    desktop_id: identifierSchema,
    tab_id: identifierSchema,
  })
  .transform((window): WindowManagerLayoutWindow => ({
    id: window.id,
    app: window.app,
    instanceKey: window.instance_key ?? null,
    route: window.route,
    navStack: window.nav_stack,
    desktopId: window.desktop_id,
    tabId: window.tab_id,
  }));

const documentFields = {
  version: z.literal(6),
  workspace_id: z.string(),
  desktops: z.array(desktopSchema).min(1),
  windows: z.record(z.string(), layoutWindowSchema),
  overrides: z.record(z.string(), z.unknown()),
};

function toDocument(document: {
  version: 6;
  workspace_id: string;
  desktops: WindowManagerLayoutDesktop[];
  windows: Record<string, WindowManagerLayoutWindow>;
  overrides: Record<string, unknown>;
}): WindowManagerLayoutDocument {
  return {
    version: document.version,
    workspaceId: document.workspace_id,
    desktops: document.desktops,
    windows: document.windows,
    overrides: document.overrides,
  };
}

export const windowManagerLayoutDocumentSchema = z
  .strictObject(documentFields)
  .transform(toDocument);

const diagnosticSchema = z
  .strictObject({
    code: z.string(),
    path: z.string().optional(),
    message: z.string(),
  })
  .transform((diagnostic): WindowManagerLayoutDiagnostic => ({
    code: diagnostic.code,
    path: diagnostic.path ?? null,
    message: diagnostic.message,
  }));

const changesSchema = z
  .object({
    desktop_ids: z.array(identifierSchema).nullish(),
    window_ids: z.array(identifierSchema).nullish(),
    tab_ids: z.array(identifierSchema).nullish(),
    node_ids: z.array(identifierSchema).nullish(),
    client_ids: z.array(identifierSchema).nullish(),
    tab_split: z.array(identifierSchema).nullish(),
    tab_unsplit: z.array(identifierSchema).nullish(),
  })
  .transform((changes): WindowManagerLayoutChanges => ({
    desktopIds: changes.desktop_ids ?? [],
    windowIds: changes.window_ids ?? [],
    tabIds: changes.tab_ids ?? [],
    nodeIds: changes.node_ids ?? [],
    clientIds: changes.client_ids ?? [],
    tabSplit: changes.tab_split ?? [],
    tabUnsplit: changes.tab_unsplit ?? [],
  }));

/**
 * A stored profile document is read in the version it was saved in. Version 6
 * parses into the editor's model; versions 4 and 5 stay in wire form, so no
 * second converter lives here — the daemon converts them when the editor opens
 * one (`convertWindowManagerLayoutDocument`).
 */
const profileSourceSchema = z
  .record(z.string(), z.unknown())
  .transform((wire, context): WindowManagerLayoutProfileSource => {
    const version = wire.version;
    if (version === 6) {
      const parsed = windowManagerLayoutDocumentSchema.safeParse(wire);
      if (parsed.success) return { kind: "current", document: parsed.data };
      for (const issue of parsed.error.issues) {
        context.addIssue({ code: "custom", message: issue.message, path: issue.path });
      }
      return z.NEVER;
    }
    if (typeof version === "number" && EARLIER_LAYOUT_VERSIONS.has(version)) {
      return { kind: "earlier", version, wire };
    }
    context.addIssue({ code: "custom", message: `unsupported layout version ${String(version)}` });
    return z.NEVER;
  });

const profileSchema = z
  .strictObject({
    version: z.literal(1),
    id: identifierSchema,
    display_name: z.string().trim().min(1),
    aspect_variant: z.enum(["any", "portrait", "landscape"]),
    participant_slots: z.array(identifierSchema).nullish(),
    overflow_policy: z.enum(["stack", "reject"]),
    document: profileSourceSchema,
  })
  .transform(profile => ({
    version: profile.version,
    id: profile.id,
    displayName: profile.display_name,
    aspectVariant: profile.aspect_variant,
    participantSlots: profile.participant_slots ?? [],
    overflowPolicy: profile.overflow_policy,
    source: profile.document,
  }));

const resourceRecordSchema = z
  .strictObject({
    kind: z.literal("window_layout"),
    id: identifierSchema,
    version: z.number().int().positive(),
    scope: z.strictObject({
      kind: z.enum(["user", "workspace"]),
      id: z.string().optional(),
    }),
    owner: z.unknown(),
    source: z.unknown(),
    spec: profileSchema,
    created_at: z.string(),
    updated_at: z.string(),
  })
  .transform((record): WindowManagerLayoutResourceRecord => ({
    kind: record.kind,
    id: record.id,
    version: record.version,
    scope: { kind: record.scope.kind, id: record.scope.id ?? "" },
    spec: record.spec,
    createdAt: record.created_at,
    updatedAt: record.updated_at,
  }));

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

/**
 * The snapshot a preview returns carries the document plus its runtime fields
 * (closed entries, migration, timestamps); only the revision and the document
 * are read. For a `layout.replace` this document is the daemon's version 6
 * result for whatever version was sent, which is how an earlier document is
 * converted without a converter in the web.
 */
export function parseWindowManagerLayoutPreview(value: unknown): WindowManagerLayoutPreview {
  return z
    .object({
      snapshot: z.object({ revision: safeRevisionSchema, ...documentFields }),
      changed: z.boolean(),
      changes: changesSchema,
      diagnostics: z.array(diagnosticSchema).nullish(),
    })
    .transform((preview): WindowManagerLayoutPreview => ({
      revision: preview.snapshot.revision,
      changed: preview.changed,
      changes: preview.changes,
      diagnostics: preview.diagnostics ?? [],
      document: toDocument(preview.snapshot),
    }))
    .parse(value);
}

export function parseWindowManagerLayoutApply(value: unknown): WindowManagerLayoutApplyResult {
  return z
    .object({
      snapshot: z.object({ revision: safeRevisionSchema }),
      applied: z.boolean(),
      diagnostics: z.array(diagnosticSchema).nullish(),
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

function nodeToWire(node: WindowManagerLayoutNode): Record<string, unknown> {
  if (node.kind === "leaf") return { id: node.id, kind: node.kind, window_id: node.windowId };
  return {
    id: node.id,
    kind: node.kind,
    axis: node.axis,
    children: node.children.map(nodeToWire),
    weights: [...node.weights],
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
      active_tab_id: desktop.activeTabId,
      tabs: desktop.tabs.map(tab => ({
        id: tab.id,
        pinned: tab.pinned,
        root: nodeToWire(tab.root),
      })),
    })),
    windows: Object.fromEntries(
      Object.entries(document.windows).map(([id, window]) => [
        id,
        {
          id: window.id,
          app: window.app,
          ...(window.instanceKey ? { instance_key: window.instanceKey } : {}),
          route: window.route,
          nav_stack: window.navStack,
          desktop_id: window.desktopId,
          tab_id: window.tabId,
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
