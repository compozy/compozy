export type WindowManagerLayoutAxis = "horizontal" | "vertical";
export type WindowManagerLayoutAspect = "any" | "portrait" | "landscape";
export type WindowManagerLayoutOverflow = "stack" | "reject";
export type WindowManagerLayoutScopeKind = "global" | "workspace";

export interface WindowManagerNormalizedRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

export type WindowManagerLayoutNode =
  | {
      id: string;
      kind: "leaf";
      windowId: string;
    }
  | {
      id: string;
      kind: "stack";
      windowIds: string[];
      activeId: string;
    }
  | {
      id: string;
      kind: "split";
      axis: WindowManagerLayoutAxis;
      children: WindowManagerLayoutNode[];
      weights: number[];
    };

export interface WindowManagerLayoutGroup {
  id: string;
  frame: WindowManagerNormalizedRect;
  root: WindowManagerLayoutNode;
}

export interface WindowManagerLayoutFloatingStack {
  id: string;
  windowIds: string[];
  activeId: string | null;
  rect: WindowManagerNormalizedRect;
  minimized: boolean;
}

export interface WindowManagerLayoutDesktop {
  id: string;
  name: string;
  order: number;
  groups: WindowManagerLayoutGroup[];
  floating: string[];
  floatingStacks: WindowManagerLayoutFloatingStack[];
}

export interface WindowManagerLayoutWindow {
  id: string;
  app: string;
  instanceKey: string | null;
  route: {
    pathname: string;
    search: Record<string, unknown>;
  };
  navStack: {
    pathname: string;
    search: Record<string, unknown>;
  }[];
  pinned: boolean;
  placement: "tiled" | "stacked" | "floating";
  desktopId: string;
  floatingRect: WindowManagerNormalizedRect;
  minimized: boolean;
  zoomed: boolean;
  returnAnchor: unknown | null;
}

export interface WindowManagerLayoutDocument {
  version: 4;
  workspaceId: string;
  desktops: WindowManagerLayoutDesktop[];
  windows: Record<string, WindowManagerLayoutWindow>;
  overrides: Record<string, unknown>;
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
  groupIds: string[];
  nodeIds: string[];
  clientIds: string[];
  stackGrouped: string[];
  stackUngrouped: string[];
}

export interface WindowManagerLayoutPreview {
  revision: number;
  changed: boolean;
  changes: WindowManagerLayoutChanges;
  diagnostics: WindowManagerLayoutDiagnostic[];
}

export interface WindowManagerLayoutApplyResult {
  revision: number;
  applied: boolean;
  diagnostics: WindowManagerLayoutDiagnostic[];
}

export interface WindowManagerLayoutProfile {
  version: 1;
  id: string;
  displayName: string;
  aspectVariant: WindowManagerLayoutAspect;
  participantSlots: string[];
  overflowPolicy: WindowManagerLayoutOverflow;
  document: WindowManagerLayoutDocument;
}

export interface WindowManagerLayoutResourceRecord {
  kind: "window_layout";
  id: string;
  version: number;
  scope: {
    kind: WindowManagerLayoutScopeKind;
    id: string;
  };
  spec: WindowManagerLayoutProfile;
  createdAt: string;
  updatedAt: string;
}
