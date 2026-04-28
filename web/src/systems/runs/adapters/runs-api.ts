import { apiErrorMessage, daemonApiClient, requireData } from "@/lib/api-client";
import { ACTIVE_WORKSPACE_HEADER } from "@/systems/app-shell";

import type { Run, RunListParams, RunSnapshot, RunTranscript, TaskRunRequestBody } from "../types";

function normalizeStatus(status?: RunListParams["status"]): string | undefined {
  if (!status || status === "all") {
    return undefined;
  }
  return status;
}

function normalizeMode(mode?: RunListParams["mode"]): string | undefined {
  if (!mode || mode === "all") {
    return undefined;
  }
  return mode;
}

export async function listRuns(params: RunListParams): Promise<Run[]> {
  const query: Record<string, string | number> = {};
  if (params.workspaceId) {
    query.workspace = params.workspaceId;
  }
  const status = normalizeStatus(params.status);
  if (status) {
    query.status = status;
  }
  const mode = normalizeMode(params.mode);
  if (mode) {
    query.mode = mode;
  }
  if (params.limit && params.limit > 0) {
    query.limit = params.limit;
  }
  const { data, error, response } = await daemonApiClient.GET("/api/runs", {
    params: {
      query: Object.keys(query).length > 0 ? query : undefined,
    },
  });
  const payload = requireData(data, response, "Failed to load runs", error);
  return payload.runs ?? [];
}

export async function getRun(runId: string): Promise<Run> {
  const { data, error, response } = await daemonApiClient.GET("/api/runs/{run_id}", {
    params: { path: { run_id: runId } },
  });
  const payload = requireData(data, response, `Failed to load run ${runId}`, error);
  return payload.run;
}

export async function getRunSnapshot(runId: string): Promise<RunSnapshot> {
  const { data, error, response } = await daemonApiClient.GET("/api/runs/{run_id}/snapshot", {
    params: { path: { run_id: runId } },
  });
  return requireData(data, response, `Failed to load run snapshot ${runId}`, error);
}

export async function getRunTranscript(runId: string): Promise<RunTranscript> {
  const { data, error, response } = await daemonApiClient.GET("/api/runs/{run_id}/transcript", {
    params: { path: { run_id: runId } },
  });
  return requireData(data, response, `Failed to load run transcript ${runId}`, error);
}

export interface CancelRunParams {
  runId: string;
}

export async function cancelRun(params: CancelRunParams): Promise<void> {
  const { data, error, response } = await daemonApiClient.POST("/api/runs/{run_id}/cancel", {
    params: { path: { run_id: params.runId } },
  });
  if (error || !response.ok) {
    throw new Error(apiErrorMessage(error, `Failed to cancel run ${params.runId}`));
  }
  if (data === undefined) {
    throw new Error(`Failed to cancel run ${params.runId}: empty response`);
  }
}

export interface StartWorkflowRunParams {
  workspaceId: string;
  slug: string;
  body?: TaskRunRequestBody;
}

export async function startWorkflowRun(params: StartWorkflowRunParams): Promise<Run> {
  const { data, error, response } = await daemonApiClient.POST("/api/tasks/{slug}/runs", {
    params: {
      path: { slug: params.slug },
      header: { [ACTIVE_WORKSPACE_HEADER]: params.workspaceId },
    },
    body: {
      ...params.body,
      workspace: params.workspaceId,
    },
  });
  if (!data) {
    throw new Error(apiErrorMessage(error, `Failed to start run for ${params.slug}`));
  }
  const payload = requireData(data, response, `Failed to start run for ${params.slug}`, error);
  return payload.run;
}
