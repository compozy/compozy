/**
 * Layout lifecycle of the mounted flow (`use-layout.ts`), as a pure reducer so its ordering rules
 * are testable: every run gets an increasing request id, and only the latest request for the
 * mounted flow may write positions. Positions of another flow are never kept.
 */
export type Point = { x: number; y: number };
export type Box = Point & { width: number; height: number };
export type EdgeRoute = { points: Point[]; label: Box | null };
export type Layout = {
  nodes: Record<string, Point>;
  edges: Record<string, EdgeRoute>;
  width: number;
  height: number;
};

export type LayoutStatus = "measuring" | "laying-out" | "ready" | "failed";

export type LayoutState = {
  flow: string;
  /** Id of the latest request; responses for older ids are ignored. */
  request: number;
  status: LayoutStatus;
  /** Positions of the mounted flow (kept while a newer request for the same flow runs). */
  layout: Layout | null;
};

export type LayoutAction =
  | { type: "reset"; flow: string }
  | { type: "request"; flow: string }
  | { type: "resolve"; flow: string; request: number; layout: Layout }
  | { type: "reject"; flow: string; request: number }
  | { type: "retry" };

export function initialLayout(flow: string): LayoutState {
  return { flow, request: 0, status: "measuring", layout: null };
}

export function layoutReducer(state: LayoutState, action: LayoutAction): LayoutState {
  switch (action.type) {
    case "reset":
      // Request ids keep increasing across flows, so a late answer for the old flow never matches.
      return action.flow === state.flow
        ? state
        : { flow: action.flow, request: state.request, status: "measuring", layout: null };
    case "request":
      if (action.flow !== state.flow) return state;
      return { ...state, request: state.request + 1, status: "laying-out" };
    case "retry":
      if (state.status !== "failed") return state;
      return { ...state, request: state.request + 1, status: "laying-out" };
    case "resolve":
      if (action.flow !== state.flow || action.request !== state.request) return state;
      return { ...state, status: "ready", layout: action.layout };
    case "reject":
      if (action.flow !== state.flow || action.request !== state.request) return state;
      return { ...state, status: "failed" };
  }
}
