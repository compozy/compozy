import {
  apiClient,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";

import type {
  SkillActionResponse,
  SkillMarketplaceInstallPayload,
  SkillMarketplaceInstallRequest,
  SkillMarketplaceRemovePayload,
  SkillMarketplaceUpdatePayload,
  SkillMarketplaceUpdateRequest,
  SkillExposeRequest,
  SkillExposeFailureResponse,
  SkillExposeResponse,
  SkillExposeResult,
  SkillPayload,
  SkillShadowsResponse,
  SkillUnexposeResponse,
} from "../types";

export class SkillApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "SkillApiError";
  }
}

/**
 * Every expose/unexpose failure — single- or multi-target — answers with one
 * envelope carrying per-target results. The surface renders each target's own
 * daemon code, so the failure travels with the error instead of collapsing into
 * a status number.
 */
export class SkillExposeError extends SkillApiError {
  constructor(
    message: string,
    status: number,
    public readonly code: string,
    public readonly results: SkillExposeResult[],
    public readonly rolledBack: boolean
  ) {
    super(message, status);
    this.name = "SkillExposeError";
  }
}

function isSkillExposeFailureResponse(value: unknown): value is SkillExposeFailureResponse {
  if (value == null || typeof value !== "object") return false;
  const summary = Reflect.get(value, "error");
  if (summary == null || typeof summary !== "object") return false;
  return (
    typeof Reflect.get(summary, "code") === "string" &&
    typeof Reflect.get(summary, "message") === "string" &&
    Array.isArray(Reflect.get(value, "results"))
  );
}

function exposeFailure(verb: string, name: string, status: number, error: unknown): never {
  const body = isSkillExposeFailureResponse(error) ? error : undefined;
  const code = body?.error.code ?? "expose_failed";
  const message =
    typeof body?.error.message === "string" && body.error.message.trim() !== ""
      ? body.error.message
      : `Failed to ${verb} skill "${name}": ${status}`;
  const results = Array.isArray(body?.results) ? body.results : [];
  throw new SkillExposeError(message, status, code, results, body?.rolled_back === true);
}

export async function exposeSkill(
  name: string,
  body: SkillExposeRequest,
  profile?: string,
  signal?: AbortSignal
): Promise<SkillExposeResponse> {
  const { data, error, response } = await apiClient.POST("/api/skills/{name}/expose", {
    params: { path: { name }, query: { profile } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    exposeFailure("expose", name, response.status, error);
  }
  return requireResponseData(data, response, `Failed to expose skill "${name}"`);
}

export async function unexposeSkill(
  name: string,
  body: SkillExposeRequest,
  profile?: string,
  signal?: AbortSignal
): Promise<SkillUnexposeResponse> {
  const { data, error, response } = await apiClient.POST("/api/skills/{name}/unexpose", {
    params: { path: { name }, query: { profile } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    exposeFailure("unexpose", name, response.status, error);
  }
  return requireResponseData(data, response, `Failed to unexpose skill "${name}"`);
}

export async function listSkills(
  workspace: string,
  signal?: AbortSignal,
  profile?: string
): Promise<SkillPayload[]> {
  const { data, error, response } = await apiClient.GET("/api/skills", {
    params: { query: { workspace, profile } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    throw new SkillApiError(
      defaultApiErrorMessage("Failed to fetch skills", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to fetch skills").skills;
}

export async function getSkill(
  name: string,
  workspace: string,
  signal?: AbortSignal,
  profile?: string
): Promise<SkillPayload> {
  // Detail reads take the canonical workspace id; list/content/shadows keep `workspace`.
  const { data, error, response } = await apiClient.GET("/api/skills/{name}", {
    params: {
      path: { name },
      query: { workspace_id: workspace, profile },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(`Skill not found: ${name}`, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage(`Failed to fetch skill "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch skill "${name}"`).skill;
}

export async function getSkillContent(
  name: string,
  workspace: string,
  signal?: AbortSignal,
  profile?: string
): Promise<string> {
  const { data, error, response } = await apiClient.GET("/api/skills/{name}/content", {
    params: {
      path: { name },
      query: { workspace, profile },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(`Skill not found: ${name}`, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage(`Failed to fetch skill content "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to fetch skill content "${name}"`).content;
}

export async function getSkillShadows(
  name: string,
  workspace: string,
  signal?: AbortSignal,
  profile?: string
): Promise<SkillShadowsResponse> {
  const { data, error, response } = await apiClient.GET("/api/skills/{name}/shadows", {
    params: {
      path: { name },
      query: { workspace, profile },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError("Skill not found: " + name, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage('Failed to fetch skill resolution "' + name + '"', response, error),
      response.status
    );
  }
  return requireResponseData(data, response, 'Failed to fetch skill resolution "' + name + '"');
}

export async function enableSkill(
  name: string,
  workspace: string,
  profile?: string,
  signal?: AbortSignal
): Promise<SkillActionResponse> {
  const { data, error, response } = await apiClient.POST("/api/skills/{name}/enable", {
    params: {
      path: { name },
      query: { workspace, profile },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(`Skill not found: ${name}`, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage(`Failed to enable skill "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to enable skill "${name}"`);
}

export async function disableSkill(
  name: string,
  workspace: string,
  profile?: string,
  signal?: AbortSignal
): Promise<SkillActionResponse> {
  const { data, error, response } = await apiClient.POST("/api/skills/{name}/disable", {
    params: {
      path: { name },
      query: { workspace, profile },
    },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(`Skill not found: ${name}`, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage(`Failed to disable skill "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to disable skill "${name}"`);
}

export async function installSkillMarketplace(
  body: SkillMarketplaceInstallRequest,
  signal?: AbortSignal
): Promise<SkillMarketplaceInstallPayload> {
  const { data, error, response } = await apiClient.POST("/api/skills/marketplace/install", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(`Marketplace skill not found: ${body.slug}`, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage(`Failed to install marketplace skill "${body.slug}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to install marketplace skill "${body.slug}"`)
    .skill;
}

export async function updateSkillMarketplace(
  body: SkillMarketplaceUpdateRequest,
  signal?: AbortSignal
): Promise<SkillMarketplaceUpdatePayload[]> {
  const { data, error, response } = await apiClient.POST("/api/skills/marketplace/update", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(
        `Installed marketplace skill not found: ${body.name ?? "<all>"}`,
        404
      );
    }
    throw new SkillApiError(
      defaultApiErrorMessage("Failed to update marketplace skills", response, error),
      response.status
    );
  }
  return requireResponseData(data, response, "Failed to update marketplace skills").skills;
}

export async function removeSkillMarketplace(
  name: string,
  signal?: AbortSignal
): Promise<SkillMarketplaceRemovePayload> {
  const { data, error, response } = await apiClient.DELETE("/api/skills/marketplace/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) {
    if (response.status === 404) {
      throw new SkillApiError(`Installed marketplace skill not found: ${name}`, 404);
    }
    throw new SkillApiError(
      defaultApiErrorMessage(`Failed to remove marketplace skill "${name}"`, response, error),
      response.status
    );
  }
  return requireResponseData(data, response, `Failed to remove marketplace skill "${name}"`).skill;
}
