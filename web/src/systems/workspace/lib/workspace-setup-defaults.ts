import type { AgentPayload } from "@/systems/agent";

export type WorkspaceSetupCollection<T> =
  | { state: "loading" }
  | { state: "error"; message: string }
  | { state: "ready"; entries: T[] };

export interface WorkspaceSetupDefaultsModel {
  agents: WorkspaceSetupCollection<AgentPayload>;
}
