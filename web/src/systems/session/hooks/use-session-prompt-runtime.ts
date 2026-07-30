import { useRef, useState } from "react";

import { isReasoningEffort, type RuntimeSpeed } from "@/lib/api-contract";
import { resolveAgentRuntimeValue, useAgents } from "@/systems/agent";
import { useRuntimeModelCatalog, type RuntimeCatalogProvider } from "@/systems/model-catalog";
import type { RuntimeProviderOption, RuntimeSelectorValue } from "@/systems/runtime";
import { useWorkspace, type SessionProviderOption } from "@/systems/workspace";

import type {
  SessionPromptRuntimeContextValue,
  SessionPromptRuntimeSnapshot,
} from "../contexts/session-prompt-runtime-context-value";
import type { SessionPayload } from "../types";

type SessionRuntimeEffective = NonNullable<SessionPayload["runtime"]>["effective"];

function runtimeValueFromEffective(
  effective: SessionRuntimeEffective | undefined
): RuntimeSelectorValue {
  return {
    provider: effective?.provider?.trim() ?? "",
    model: effective?.model?.trim() ?? "",
    reasoning_effort: effective?.reasoning_effort ?? "",
  };
}

function runtimeSpeedFromEffective(effective: SessionRuntimeEffective | undefined): RuntimeSpeed {
  return effective?.speed ?? "normal";
}

function snapshotFromSelection(
  value: RuntimeSelectorValue,
  speed: RuntimeSpeed
): SessionPromptRuntimeSnapshot | null {
  const provider = value.provider.trim();
  if (provider.length === 0) return null;

  const model = value.model.trim();
  return {
    provider,
    ...(model.length > 0 ? { model } : {}),
    ...(isReasoningEffort(value.reasoning_effort)
      ? { reasoning_effort: value.reasoning_effort }
      : {}),
    ...(speed === "fast" ? { speed } : {}),
  };
}

function requireSessionWorkspaceId(workspaceId: string | undefined): string {
  const normalized = workspaceId?.trim() ?? "";
  if (!normalized) {
    throw new Error("Session prompt runtime requires a non-empty workspace_id");
  }
  return normalized;
}

function runtimeProviderOptions(
  providers: SessionProviderOption[] | undefined
): RuntimeProviderOption[] {
  return (providers ?? []).map(provider => ({
    id: provider.name,
    name: provider.display_name?.trim() || provider.name,
    ...(provider.harness?.trim() ? { harness: provider.harness.trim() } : {}),
    runtime_provider: provider.runtime_provider?.trim() || provider.name,
  }));
}

/**
 * Owns the runtime intended for the next prompt. It deliberately does not mirror
 * the session's effective runtime after a user makes a choice: each queued or
 * interrupted submission snapshots this local intent at its dispatch boundary.
 */
export function useSessionPromptRuntime(
  session: SessionPayload,
  canPrompt: boolean
): SessionPromptRuntimeContextValue {
  const workspaceId = requireSessionWorkspaceId(session.workspace_id);
  const workspace = useWorkspace(workspaceId);
  const agents = useAgents(workspaceId);
  const agent = agents.data?.find(candidate => candidate.name === session.agent_name);
  const effectiveRuntime = session.runtime?.effective;
  const agentRuntime = resolveAgentRuntimeValue(agent);
  const fallbackValue = runtimeValueFromEffective(effectiveRuntime);
  const defaultValue = fallbackValue.provider.length > 0 ? fallbackValue : agentRuntime;
  const defaultSpeed = runtimeSpeedFromEffective(effectiveRuntime);
  const [selectedValue, setSelectedValue] = useState<RuntimeSelectorValue | null>(null);
  const [selectedSpeed, setSelectedSpeed] = useState<RuntimeSpeed | null>(null);
  const value = selectedValue ?? defaultValue;
  const speed = selectedSpeed ?? defaultSpeed;
  const providers = runtimeProviderOptions(workspace.data?.providers);
  const catalogProviders: RuntimeCatalogProvider[] = providers.map(provider => ({
    id: provider.id,
  }));
  const catalog = useRuntimeModelCatalog(catalogProviders, {
    enabled: providers.length > 0,
  });
  const canSelectRuntime = canPrompt && !workspace.isLoading && providers.length > 0;
  const snapshot = snapshotFromSelection(value, speed);
  const snapshotRef = useRef(snapshot);
  snapshotRef.current = snapshot;

  const getRuntimeSnapshot = () => snapshotRef.current;
  const onRuntimeChange = (next: RuntimeSelectorValue) => {
    setSelectedValue(next);
  };
  const onSpeedChange = (next: RuntimeSpeed) => {
    setSelectedSpeed(next);
  };

  return {
    canSelectRuntime,
    catalog: {
      error: catalog.error,
      loaded: catalog.loaded,
      loading: catalog.loading,
      models: catalog.models,
      providers,
      refresh: catalog.refresh,
      refreshError: catalog.refreshError,
      refreshing: catalog.refreshing,
      stale: catalog.stale,
    },
    getRuntimeSnapshot,
    onRuntimeChange,
    onSpeedChange,
    speed,
    value,
  };
}
