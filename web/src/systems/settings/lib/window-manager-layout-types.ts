import type { LayoutAxis, LayoutNode, LayoutTab, WindowManagerSnapshot } from "@/systems/os";

export type WindowManagerLayoutAspect = "any" | "portrait" | "landscape";
export type WindowManagerLayoutOverflow = "stack" | "reject";
/** The daemon's resource scopes for a saved layout: user-wide or one project. */
export type WindowManagerLayoutScopeKind = "user" | "workspace";

/**
 * The editor edits the shell's own v6 topology types from the OS barrel, so a
 * tab in Settings and a tab in the shell are one shape and one renderer.
 */
export type WindowManagerLayoutDesktop = WindowManagerSnapshot["desktops"][number];
export type WindowManagerLayoutTab = LayoutTab;
export type WindowManagerLayoutNode = LayoutNode;
export type WindowManagerLayoutSplit = Extract<LayoutNode, { kind: "split" }>;
export type WindowManagerLayoutLeaf = Extract<LayoutNode, { kind: "leaf" }>;
export type WindowManagerLayoutAxis = LayoutAxis;
export type WindowManagerLayoutWindow = WindowManagerSnapshot["windows"][string];
export type WindowManagerLayoutRoute = WindowManagerLayoutWindow["route"];

/** A side of a window a moved window lands on; it picks the split axis and the order. */
export type WindowManagerLayoutEdge = "left" | "right" | "top" | "bottom";

/**
 * The version 6 layout document: the snapshot without revision, history and
 * closed entries. `overrides` stays in wire form (snake_case), because the
 * editor never edits it and must send it back exactly as the daemon gave it.
 */
export interface WindowManagerLayoutDocument {
  version: 6;
  workspaceId: string;
  desktops: readonly WindowManagerLayoutDesktop[];
  windows: Readonly<Record<string, WindowManagerLayoutWindow>>;
  overrides: Readonly<Record<string, unknown>>;
}

export interface WindowManagerLayoutState {
  revision: number;
  document: WindowManagerLayoutDocument;
}

export interface WindowManagerLayoutDiagnostic {
  code: string;
  path: string | null;
  message: string;
}

export interface WindowManagerLayoutValidation {
  workspaceId: string;
  valid: boolean;
  diagnostics: WindowManagerLayoutDiagnostic[];
}

export interface WindowManagerLayoutChanges {
  desktopIds: string[];
  windowIds: string[];
  tabIds: string[];
  nodeIds: string[];
  clientIds: string[];
  tabSplit: string[];
  tabUnsplit: string[];
}

export interface WindowManagerLayoutPreview {
  revision: number;
  changed: boolean;
  changes: WindowManagerLayoutChanges;
  diagnostics: WindowManagerLayoutDiagnostic[];
  /** The layout the daemon would hold after the replace: version 6 whatever version was sent. */
  document: WindowManagerLayoutDocument;
}

export interface WindowManagerLayoutApplyResult {
  revision: number;
  applied: boolean;
  diagnostics: WindowManagerLayoutDiagnostic[];
}

/**
 * A saved profile's document as the daemon stored it. Profiles saved before
 * tabs and splits can still hold a version 4 or 5 document; the daemon converts
 * it on read, so the editor keeps the stored form untouched until it asks the
 * daemon for the version 6 result (US-022.AC-3).
 */
export type WindowManagerLayoutProfileSource =
  | { kind: "current"; document: WindowManagerLayoutDocument }
  | { kind: "earlier"; version: number; wire: Readonly<Record<string, unknown>> };

/** What Settings writes: always a version 6 document. */
export interface WindowManagerLayoutProfile {
  version: 1;
  id: string;
  displayName: string;
  aspectVariant: WindowManagerLayoutAspect;
  participantSlots: string[];
  overflowPolicy: WindowManagerLayoutOverflow;
  document: WindowManagerLayoutDocument;
}

export interface WindowManagerLayoutProfileSpec extends Omit<
  WindowManagerLayoutProfile,
  "document"
> {
  source: WindowManagerLayoutProfileSource;
}

export interface WindowManagerLayoutResourceRecord {
  kind: "window_layout";
  id: string;
  version: number;
  scope: {
    kind: WindowManagerLayoutScopeKind;
    id: string;
  };
  spec: WindowManagerLayoutProfileSpec;
  createdAt: string;
  updatedAt: string;
}
