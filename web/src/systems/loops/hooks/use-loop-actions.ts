import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  approveLoopRun,
  createLoop,
  deleteLoop,
  patchLoop,
  pauseLoopRun,
  putLoopAnnotations,
  putLoopConfig,
  resumeLoopRun,
  runLoop,
  stopLoopRun,
  validateLoop,
} from "../adapters/loops-api";
import { loopsKeys } from "../lib/query-keys";
import type {
  ApproveLoopRunRequest,
  CreateLoopRequest,
  LoopAnnotationsUpdateRequest,
  LoopConfigUpdateRequest,
  PatchLoopRequest,
  RunLoopRequest,
  ValidateLoopRequest,
} from "../types";

type QueryClient = ReturnType<typeof useQueryClient>;

interface WorkspaceParams {
  workspaceId: string;
}

interface LoopNameParams extends WorkspaceParams {
  name: string;
}

interface LoopRunParams extends WorkspaceParams {
  runId: string;
}

interface CreateLoopParams extends WorkspaceParams {
  data: CreateLoopRequest;
}

interface PatchLoopParams extends LoopNameParams {
  data: PatchLoopRequest;
}

interface RunLoopParams extends LoopNameParams {
  data: RunLoopRequest;
  dry?: boolean;
}

interface ValidateLoopParams extends LoopNameParams {
  data: ValidateLoopRequest;
}

interface PutLoopConfigParams extends LoopNameParams {
  data: LoopConfigUpdateRequest;
}

interface PutLoopAnnotationsParams extends LoopNameParams {
  data: LoopAnnotationsUpdateRequest;
}

interface ApproveLoopRunParams extends LoopRunParams {
  data: ApproveLoopRunRequest;
}

/** Invalidate the catalog + (optionally) a single loop's detail/config/annotations. */
function invalidateLoopDefinitionQueries(
  queryClient: QueryClient,
  workspaceId: string,
  name?: string
) {
  const pending = [
    queryClient.invalidateQueries({ queryKey: loopsKeys.catalogByWorkspace(workspaceId) }),
  ];
  if (name) {
    pending.push(queryClient.invalidateQueries({ queryKey: loopsKeys.detail(workspaceId, name) }));
    pending.push(queryClient.invalidateQueries({ queryKey: loopsKeys.config(workspaceId, name) }));
    pending.push(
      queryClient.invalidateQueries({ queryKey: loopsKeys.annotations(workspaceId, name) })
    );
  }
  return Promise.all(pending);
}

/** Invalidate this workspace's runs lists + (optionally) a single run's detail. */
function invalidateLoopRunQueries(queryClient: QueryClient, workspaceId: string, runId?: string) {
  const pending = [
    queryClient.invalidateQueries({ queryKey: loopsKeys.catalogByWorkspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) }),
  ];
  if (runId) {
    pending.push(
      queryClient.invalidateQueries({ queryKey: loopsKeys.runDetail(workspaceId, runId) })
    );
  }
  return Promise.all(pending);
}

export function useCreateLoop() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, data }: CreateLoopParams) => createLoop(workspaceId, data),
    onSettled: (result, _error, { workspaceId }) =>
      invalidateLoopDefinitionQueries(queryClient, workspaceId, result?.name),
  });
}

export function usePatchLoop() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name, data }: PatchLoopParams) =>
      patchLoop(workspaceId, name, data),
    onSettled: (_result, _error, { workspaceId, name }) =>
      invalidateLoopDefinitionQueries(queryClient, workspaceId, name),
  });
}

export function useDeleteLoop() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name }: LoopNameParams) => deleteLoop(workspaceId, name),
    onSuccess: (_result, { workspaceId, name }) => {
      queryClient.removeQueries({ queryKey: loopsKeys.detail(workspaceId, name) });
      queryClient.removeQueries({ queryKey: loopsKeys.config(workspaceId, name) });
      queryClient.removeQueries({ queryKey: loopsKeys.annotations(workspaceId, name) });
      return queryClient.invalidateQueries({
        queryKey: loopsKeys.catalogByWorkspace(workspaceId),
      });
    },
  });
}

export function useValidateLoop() {
  return useMutation({
    mutationFn: ({ workspaceId, name, data }: ValidateLoopParams) =>
      validateLoop(workspaceId, name, data),
  });
}

export function useRunLoop() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name, data, dry }: RunLoopParams) =>
      runLoop(workspaceId, name, data, { dry }),
    onSettled: (_result, _error, { workspaceId, dry }) => {
      // A dry run creates no durable state, so it never invalidates run caches.
      if (dry) {
        return undefined;
      }
      return invalidateLoopRunQueries(queryClient, workspaceId);
    },
  });
}

export function usePutLoopConfig() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name, data }: PutLoopConfigParams) =>
      putLoopConfig(workspaceId, name, data),
    onSettled: (_result, _error, { workspaceId, name }) =>
      queryClient.invalidateQueries({ queryKey: loopsKeys.config(workspaceId, name) }),
  });
}

export function usePutLoopAnnotations() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, name, data }: PutLoopAnnotationsParams) =>
      putLoopAnnotations(workspaceId, name, data),
    onSettled: (_result, _error, { workspaceId, name }) =>
      queryClient.invalidateQueries({ queryKey: loopsKeys.annotations(workspaceId, name) }),
  });
}

export function usePauseLoopRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId }: LoopRunParams) => pauseLoopRun(workspaceId, runId),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateLoopRunQueries(queryClient, workspaceId, runId),
  });
}

export function useResumeLoopRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId }: LoopRunParams) => resumeLoopRun(workspaceId, runId),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateLoopRunQueries(queryClient, workspaceId, runId),
  });
}

export function useStopLoopRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId }: LoopRunParams) => stopLoopRun(workspaceId, runId),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateLoopRunQueries(queryClient, workspaceId, runId),
  });
}

export function useApproveLoopRun() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ workspaceId, runId, data }: ApproveLoopRunParams) =>
      approveLoopRun(workspaceId, runId, data),
    onSettled: (_result, _error, { workspaceId, runId }) =>
      invalidateLoopRunQueries(queryClient, workspaceId, runId),
  });
}
