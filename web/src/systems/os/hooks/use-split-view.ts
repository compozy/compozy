import { createContext, use, type ReactNode } from "react";

import type { SessionPayload } from "@/systems/session";

import type { OsWindow } from "../lib/os-types";
import type { CollapsedSplit, LayoutAxis, ProjectedSeam } from "../lib/window-manager-types";

export interface SplitViewContextValue {
  windows: Readonly<Record<string, OsWindow>>;
  collapsed: ReadonlyMap<string, CollapsedSplit>;
  seams: ReadonlyMap<string, ProjectedSeam>;
  sessionOf: (win: OsWindow) => SessionPayload | undefined;
  renderWindow: (windowId: string) => ReactNode;
  renderSeam: (seam: OsSplitSeamSlot) => ReactNode;
  onShowMember: (windowId: string) => void;
  /** Pixels between two members: what each seam occupies, as the projection lays it out. */
  innerGap: number;
}

/** One seam's place in the tree, with its projection when the split is laid out. */
export interface OsSplitSeamSlot {
  splitId: string;
  boundaryIndex: number;
  axis: LayoutAxis;
  /** The inner gap the seam spans; 0 lays members edge to edge with the line drawn over them. */
  gap: number;
  seam: ProjectedSeam | undefined;
}

export const SplitViewContext = createContext<SplitViewContextValue | null>(null);

export function useSplitView(): SplitViewContextValue {
  const value = use(SplitViewContext);
  if (value === null) throw new Error("OsSplitView parts render inside <OsSplitView>.");
  return value;
}
