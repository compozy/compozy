import type { AgentPayload } from "@/systems/agent";
import type { RuntimeModelOption, RuntimeProviderOption } from "@/systems/runtime";
import type { WorktreePayload } from "@/systems/workspace";

import type { LoopInputSchema } from "../types";
import type { LoopEntityKind } from "./loop-input-kinds";

export interface LoopEntityOption {
  value: string;
  label: string;
  detail?: string;
}

export interface LoopEntityCatalog {
  options: readonly LoopEntityOption[];
  loading: boolean;
  error: string | null;
}

export interface LoopInputCatalogs {
  agents: readonly AgentPayload[];
  agentLoading: boolean;
  agentError: string | null;
  entities: Record<Exclude<LoopEntityKind, "agent">, LoopEntityCatalog>;
  worktrees: readonly WorktreePayload[];
  runtimeProviders: RuntimeProviderOption[];
  runtimeModels: RuntimeModelOption[];
  runtimeLoading: boolean;
  runtimeError: string | null;
  refreshRuntime: () => void;
  refreshingRuntime: boolean;
}

export interface LoopInputCatalogNeeds {
  entities: ReadonlySet<LoopEntityKind>;
  runtime: boolean;
}

export function loopInputCatalogNeeds(schema: LoopInputSchema | undefined): LoopInputCatalogNeeds {
  const entities = new Set<LoopEntityKind>();
  let runtime = false;
  for (const field of Object.values(schema ?? {})) {
    if (field.type === "agent") entities.add("agent");
    if (field.type === "runtime") runtime = true;
    if (field.type === "ref" && field.ref?.kind) entities.add(field.ref.kind);
  }
  return { entities, runtime };
}
