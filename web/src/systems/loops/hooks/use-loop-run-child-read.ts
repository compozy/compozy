import { createContext, use } from "react";

/**
 * What a child-run read needs from the page it sits on: the workspace that
 * scopes it, the page clock its elapsed times tick against, and whether that
 * clock is still ticking.
 *
 * Child runs surface in the Progress steps, the Inspect graph and the node
 * panel, several components below the page. The page provides this once rather
 * than threading it through every register in between.
 */
export interface LoopRunChildRead {
  workspaceId: string;
  nowMs: number;
  /**
   * True while the page's own clock ticks. A detached child can outlive its
   * parent; once the parent's clock stops, a live child keeps its own.
   */
  clockLive: boolean;
}

export const LoopRunChildReadContext = createContext<LoopRunChildRead>({
  workspaceId: "",
  nowMs: 0,
  clockLive: false,
});

/** The workspace and clock the run page provides to every child-run read below it. */
export function useLoopRunChildRead(): LoopRunChildRead {
  return use(LoopRunChildReadContext);
}
