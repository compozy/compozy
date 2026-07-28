import { useState } from "react";

import type { SessionPayload } from "@/systems/session";

import {
  useAgentHeartbeat,
  useAgentHeartbeatHistory,
  useAgentHeartbeatStatus,
  usePutAgentHeartbeat,
  useRollbackAgentHeartbeat,
  useValidateAgentHeartbeat,
  useWakeAgentHeartbeat,
} from "./use-agent-heartbeat";
import {
  useAgentSoul,
  useAgentSoulHistory,
  usePutAgentSoul,
  useRollbackAgentSoul,
  useValidateAgentSoul,
} from "./use-agent-soul";
import { formatPromptWordCount } from "../lib/agent-absent-value";
import type { AgentInstructionFile } from "../lib/agent-detail-search";
import type { AgentPayload } from "../types";
import { buildAuthoredFileResourceKey } from "./use-agent-authored-file-editor";

export interface UseAgentInstructionsTabArgs {
  agent: AgentPayload;
  file: AgentInstructionFile;
  workspaceId: string | null;
  sessions: SessionPayload[];
}

export function useAgentInstructionsTab({
  agent,
  file,
  workspaceId,
  sessions,
}: UseAgentInstructionsTabArgs) {
  // Both reads stay active while Instructions is mounted so missing badges are truthful
  // before the operator visits either authored-file tab.
  const soulQuery = useAgentSoul(agent.name, workspaceId);
  const soulHistory = useAgentSoulHistory(agent.name, workspaceId, { enabled: file === "soul" });
  const putSoul = usePutAgentSoul(agent.name, workspaceId);
  const validateSoul = useValidateAgentSoul(agent.name);
  const rollbackSoul = useRollbackAgentSoul(agent.name, workspaceId);

  const heartbeatQuery = useAgentHeartbeat(agent.name, workspaceId);
  const heartbeatHistory = useAgentHeartbeatHistory(agent.name, workspaceId, {
    enabled: file === "heartbeat",
  });
  const putHeartbeat = usePutAgentHeartbeat(agent.name, workspaceId);
  const validateHeartbeat = useValidateAgentHeartbeat(agent.name);
  const rollbackHeartbeat = useRollbackAgentHeartbeat(agent.name, workspaceId);
  const wakeHeartbeat = useWakeAgentHeartbeat(agent.name, workspaceId);

  const activeSessions = sessions.filter(session => session.state === "active");
  const [requestedWakeSessionId, setWakeSessionId] = useState<string | null>(null);
  const requestedSessionIsActive =
    requestedWakeSessionId === null ||
    activeSessions.some(session => session.id === requestedWakeSessionId);
  if (!requestedSessionIsActive) {
    setWakeSessionId(null);
  }
  const wakeSessionId =
    activeSessions.length === 1
      ? (activeSessions[0]?.id ?? null)
      : requestedSessionIsActive
        ? requestedWakeSessionId
        : null;

  const statusQuery = useAgentHeartbeatStatus(
    agent.name,
    {
      workspaceId: workspaceId ?? undefined,
      sessionId: wakeSessionId ?? undefined,
      includeRecentWakeEvents: true,
      includeSessionHealth: Boolean(wakeSessionId),
    },
    { enabled: file === "heartbeat" }
  );

  const soulMissing =
    soulQuery.isSuccess &&
    (soulQuery.data.validation_status === "missing" || soulQuery.data.present === false);
  const heartbeatMissing =
    heartbeatQuery.isSuccess &&
    (heartbeatQuery.data.validation_status === "missing" || heartbeatQuery.data.present === false);

  const handleWake = (sessionId: string) => {
    wakeHeartbeat.mutate({ session_id: sessionId, source: "manual" });
  };

  return {
    promptWordCount: formatPromptWordCount(agent.prompt),
    soulMissing,
    heartbeatMissing,
    soul: {
      resourceKey: buildAuthoredFileResourceKey(workspaceId, agent.name, "soul"),
      payload: soulQuery.data,
      isLoading: soulQuery.isLoading,
      isError: soulQuery.isError,
      history: soulHistory.data,
      onValidate: async (body: string) =>
        validateSoul.mutateAsync({
          body,
          ...(workspaceId ? { workspace_id: workspaceId } : {}),
        }),
      onSave: async (body: string, expectedDigest: string) => {
        const result = await putSoul.mutateAsync({
          body,
          expected_digest: expectedDigest,
          ...(workspaceId ? { workspace_id: workspaceId } : {}),
        });
        return result.soul;
      },
      onRestore: async (revisionId: string, expectedDigest: string) => {
        const result = await rollbackSoul.mutateAsync({
          revision_id: revisionId,
          expected_digest: expectedDigest,
          ...(workspaceId ? { workspace_id: workspaceId } : {}),
        });
        return result.soul;
      },
      onRetry: () => {
        void soulQuery.refetch();
        void soulHistory.refetch();
      },
    },
    heartbeat: {
      resourceKey: buildAuthoredFileResourceKey(workspaceId, agent.name, "heartbeat"),
      payload: heartbeatQuery.data,
      isLoading: heartbeatQuery.isLoading,
      isError: heartbeatQuery.isError,
      history: heartbeatHistory.data,
      onValidate: async (body: string) =>
        validateHeartbeat.mutateAsync({
          body,
          ...(workspaceId ? { workspace_id: workspaceId } : {}),
        }),
      onSave: async (body: string, expectedDigest: string) => {
        const result = await putHeartbeat.mutateAsync({
          body,
          expected_digest: expectedDigest,
          ...(workspaceId ? { workspace_id: workspaceId } : {}),
        });
        return result.heartbeat;
      },
      onRestore: async (revisionId: string, expectedDigest: string) => {
        const result = await rollbackHeartbeat.mutateAsync({
          revision_id: revisionId,
          expected_digest: expectedDigest,
          ...(workspaceId ? { workspace_id: workspaceId } : {}),
        });
        return result.heartbeat;
      },
      onRetry: () => {
        void heartbeatQuery.refetch();
        void heartbeatHistory.refetch();
        void statusQuery.refetch();
      },
      status: statusQuery.data,
      statusLoading: statusQuery.isLoading,
      statusError: statusQuery.isError,
      onRetryStatus: () => {
        void statusQuery.refetch();
      },
      activeSessions,
      wakeSessionId,
      setWakeSessionId,
      onWake: handleWake,
      waking: wakeHeartbeat.isPending,
      wakeDecision: wakeHeartbeat.data?.decision,
      wakeError:
        wakeHeartbeat.error instanceof Error
          ? wakeHeartbeat.error.message
          : wakeHeartbeat.isError
            ? "Couldn't wake session"
            : null,
    },
  };
}

export type AgentInstructionsTabViewModel = ReturnType<typeof useAgentInstructionsTab>;
