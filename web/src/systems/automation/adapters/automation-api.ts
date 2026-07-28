import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  AutomationJob,
  AutomationJobListFilter,
  AutomationJobsListResponse,
  AutomationRun,
  AutomationRunHistoryFilter,
  AutomationRunListFilter,
  AutomationTrigger,
  AutomationTriggerListFilter,
  AutomationTriggersListResponse,
  CreateAutomationJobRequest,
  CreateAutomationTriggerRequest,
  UpdateAutomationJobRequest,
  UpdateAutomationTriggerRequest,
} from "../types";

export class AutomationApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "AutomationApiError";
  }
}

function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") {
    return undefined;
  }

  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

function normalizeRunFilters(filters: AutomationRunHistoryFilter = {}): AutomationRunHistoryFilter {
  return {
    status: filters.status,
    since: normalizeOptionalText(filters.since),
    until: normalizeOptionalText(filters.until),
    limit: filters.limit,
  };
}

export async function listAutomationJobs(
  filters: AutomationJobListFilter = {},
  signal?: AbortSignal
): Promise<AutomationJobsListResponse> {
  const { data, error, response } = await apiClient.GET("/api/automation/jobs", {
    params: {
      query: {
        scope: filters.scope,
        workspace_id: normalizeOptionalText(filters.workspace_id),
        source: filters.source,
        enabled: filters.enabled,
        q: normalizeOptionalText(filters.q),
        cursor: normalizeOptionalText(filters.cursor),
        limit: filters.limit,
        loop: normalizeOptionalText(filters.loop),
      },
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to fetch automation jobs", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch automation jobs");
}

export async function getAutomationJob(id: string, signal?: AbortSignal): Promise<AutomationJob> {
  const { data, error, response } = await apiClient.GET("/api/automation/jobs/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation job not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to fetch automation job "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch automation job "${id}"`).job;
}

export async function createAutomationJob(
  body: CreateAutomationJobRequest,
  signal?: AbortSignal
): Promise<AutomationJob> {
  const { data, error, response } = await apiClient.POST("/api/automation/jobs", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to create automation job", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create automation job").job;
}

export async function updateAutomationJob(
  id: string,
  body: UpdateAutomationJobRequest,
  signal?: AbortSignal
): Promise<AutomationJob> {
  const { data, error, response } = await apiClient.PATCH("/api/automation/jobs/{id}", {
    params: { path: { id } },
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation job not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to update automation job "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to update automation job "${id}"`).job;
}

export async function deleteAutomationJob(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/automation/jobs/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation job not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to delete automation job "${id}"`, response, error),
      response.status
    );
  }
}

export async function triggerAutomationJob(
  id: string,
  signal?: AbortSignal
): Promise<AutomationRun> {
  const { data, error, response } = await apiClient.POST("/api/automation/jobs/{id}/trigger", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation job not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to trigger automation job "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to trigger automation job "${id}"`).run;
}

export async function listAutomationJobRuns(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  signal?: AbortSignal
): Promise<AutomationRun[]> {
  const { data, error, response } = await apiClient.GET("/api/automation/jobs/{id}/runs", {
    params: {
      path: { id },
      query: normalizeRunFilters(filters),
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation job not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to fetch runs for automation job "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch runs for automation job "${id}"`)
    .runs;
}

export async function listAutomationTriggers(
  filters: AutomationTriggerListFilter = {},
  signal?: AbortSignal
): Promise<AutomationTriggersListResponse> {
  const { data, error, response } = await apiClient.GET("/api/automation/triggers", {
    params: {
      query: {
        scope: filters.scope,
        workspace_id: normalizeOptionalText(filters.workspace_id),
        source: filters.source,
        enabled: filters.enabled,
        event: normalizeOptionalText(filters.event),
        q: normalizeOptionalText(filters.q),
        cursor: normalizeOptionalText(filters.cursor),
        limit: filters.limit,
        loop: normalizeOptionalText(filters.loop),
      },
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to fetch automation triggers", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch automation triggers");
}

export async function getAutomationTrigger(
  id: string,
  signal?: AbortSignal
): Promise<AutomationTrigger> {
  const { data, error, response } = await apiClient.GET("/api/automation/triggers/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation trigger not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to fetch automation trigger "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch automation trigger "${id}"`).trigger;
}

export async function createAutomationTrigger(
  body: CreateAutomationTriggerRequest,
  signal?: AbortSignal
): Promise<AutomationTrigger> {
  const { data, error, response } = await apiClient.POST("/api/automation/triggers", {
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to create automation trigger", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to create automation trigger").trigger;
}

export async function updateAutomationTrigger(
  id: string,
  body: UpdateAutomationTriggerRequest,
  signal?: AbortSignal
): Promise<AutomationTrigger> {
  const { data, error, response } = await apiClient.PATCH("/api/automation/triggers/{id}", {
    params: { path: { id } },
    body,
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation trigger not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to update automation trigger "${id}"`, response, error),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to update automation trigger "${id}"`).trigger;
}

export async function deleteAutomationTrigger(id: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/automation/triggers/{id}", {
    params: { path: { id } },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation trigger not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(`Failed to delete automation trigger "${id}"`, response, error),
      response.status
    );
  }
}

export async function listAutomationTriggerRuns(
  id: string,
  filters: AutomationRunHistoryFilter = {},
  signal?: AbortSignal
): Promise<AutomationRun[]> {
  const { data, error, response } = await apiClient.GET("/api/automation/triggers/{id}/runs", {
    params: {
      path: { id },
      query: normalizeRunFilters(filters),
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new AutomationApiError(`Automation trigger not found: ${id}`, 404);
    }

    throw new AutomationApiError(
      defaultApiErrorMessage(
        `Failed to fetch runs for automation trigger "${id}"`,
        response,
        error
      ),
      response.status
    );
  }

  return requireResponseData(data, response, `Failed to fetch runs for automation trigger "${id}"`)
    .runs;
}

export async function listAutomationRuns(
  filters: AutomationRunListFilter = {},
  signal?: AbortSignal
): Promise<AutomationRun[]> {
  const { data, error, response } = await apiClient.GET("/api/automation/runs", {
    params: {
      query: {
        job_id: normalizeOptionalText(filters.job_id),
        trigger_id: normalizeOptionalText(filters.trigger_id),
        status: filters.status,
        since: normalizeOptionalText(filters.since),
        until: normalizeOptionalText(filters.until),
        limit: filters.limit,
      },
    },
    signal,
  });

  if (apiRequestFailed(response, error)) {
    throw new AutomationApiError(
      defaultApiErrorMessage("Failed to fetch automation runs", response, error),
      response.status
    );
  }

  return requireResponseData(data, response, "Failed to fetch automation runs").runs;
}
