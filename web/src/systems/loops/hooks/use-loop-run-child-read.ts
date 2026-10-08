import { createContext, use } from "react";

/**
 * What a child-run read needs from the page it sits on: the workspace that
 * scopes it and the page clock its elapsed times tick against.
 *
 * Child runs surface in the Progress steps, the Inspect graph and the node
 * panel, several components below the page. The page provides both once rather
 * than threading them through every register in between.
 */
export interface LoopRunChildRead {
  workspaceId: string;
  nowMs: number;
}

export const LoopRunChildReadContext = createContext<LoopRunChildRead>({
  workspaceId: "",
  nowMs: 0,
});

export function useLoopRunChildRead(): LoopRunChildRead {
  return use(LoopRunChildReadContext);
}
