import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { useActiveWorkspace } from "@/systems/workspace";

import {
  putNetworkCoordination,
  putNetworkCoordinationInvitation,
  type NetworkCoordinationRef,
  type PutNetworkCoordinationInvitationRequest,
  type PutNetworkCoordinationRequest,
} from "../adapters/network-coordination-api";
import { networkCoordinationOptions, networkUsageOptions } from "../lib/query-options";
import { networkKeys } from "../lib/query-keys";
import type { NetworkUsageFilters } from "../types";

function coordinationToggleRequest(
  ref: NetworkCoordinationRef,
  expectedRevision: number
): PutNetworkCoordinationRequest {
  if (ref.scope === "task") {
    return {
      scope: "task",
      task_id: ref.taskId,
      ...(ref.runId ? { run_id: ref.runId } : {}),
      enabled: true,
      expected_revision: expectedRevision,
    };
  }
  return {
    scope: "workspace",
    ...(ref.runId ? { run_id: ref.runId } : {}),
    enabled: true,
    expected_revision: expectedRevision,
  };
}

function coordinationInvitationRequest(
  ref: NetworkCoordinationRef,
  expectedRevision: number
): PutNetworkCoordinationInvitationRequest {
  if (ref.scope === "task") {
    return {
      scope: "task",
      task_id: ref.taskId,
      ...(ref.runId ? { run_id: ref.runId } : {}),
      dismissed: true,
      expected_revision: expectedRevision,
    };
  }
  return {
    scope: "workspace",
    ...(ref.runId ? { run_id: ref.runId } : {}),
    dismissed: true,
    expected_revision: expectedRevision,
  };
}

export function useNetworkCoordination(workspaceId: string, ref: NetworkCoordinationRef) {
  return useQuery(networkCoordinationOptions(workspaceId, ref, Boolean(workspaceId)));
}

export function useNetworkUsage(filters: NetworkUsageFilters = {}) {
  const { activeWorkspaceId } = useActiveWorkspace();
  const workspaceId = activeWorkspaceId ?? "";
  return useInfiniteQuery(networkUsageOptions(workspaceId, filters, Boolean(workspaceId)));
}

export function useAcceptNetworkCoordinationInvitation(
  workspaceId: string,
  ref: NetworkCoordinationRef
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (expectedRevision: number) => {
      if (!workspaceId) {
        throw new Error("workspace is required");
      }
      return putNetworkCoordination(workspaceId, coordinationToggleRequest(ref, expectedRevision));
    },
    onSuccess: async coordination => {
      if (!workspaceId) return;
      queryClient.setQueryData(networkKeys.coordination(workspaceId, ref), coordination);
      await queryClient.invalidateQueries({
        queryKey: networkKeys.coordinationRoot(workspaceId),
      });
    },
  });
}

export function useDismissNetworkCoordinationInvitation(
  workspaceId: string,
  ref: NetworkCoordinationRef
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: async (expectedRevision: number) => {
      if (!workspaceId) {
        throw new Error("workspace is required");
      }
      return putNetworkCoordinationInvitation(
        workspaceId,
        coordinationInvitationRequest(ref, expectedRevision)
      );
    },
    onSuccess: async coordination => {
      if (!workspaceId) return;
      queryClient.setQueryData(networkKeys.coordination(workspaceId, ref), coordination);
      await queryClient.invalidateQueries({
        queryKey: networkKeys.coordinationRoot(workspaceId),
      });
    },
  });
}
