import type { Extension } from "./extension.js";
import type { ExtensionContext } from "./extension-contract.js";
import { InvalidParamsError } from "./errors.js";
import { registerProvideSurface } from "./extension-provide-surface.js";
import type {
  ForgePRSearchRequest, ForgePRReadRequest, ForgePRCandidate, ForgePRSearchResponse,
  ForgeCapabilitiesRequest,
  ForgeCapabilitiesResponse,
  ForgePRCreateRequest,
  ForgePRCreateResponse,
  ForgeStatusRequest,
  ForgeStatusResponse,
} from "./types.js";

export const FORGE_PROVIDER_CAPABILITY = "forge.provider";
export const FORGE_CAPABILITIES_METHOD = "forge/capabilities";
export const FORGE_STATUS_METHOD = "forge/status";
export const FORGE_PR_CREATE_METHOD = "forge/pr_create";

export interface ForgeProviderHandlers {
 searchPRs?: (context: ExtensionContext, request: ForgePRSearchRequest) => Promise<ForgePRSearchResponse> | ForgePRSearchResponse;
 readPR?: (context: ExtensionContext, request: ForgePRReadRequest) => Promise<ForgePRCandidate> | ForgePRCandidate;
  capabilities: (
    context: ExtensionContext,
    request: ForgeCapabilitiesRequest
  ) => Promise<ForgeCapabilitiesResponse> | ForgeCapabilitiesResponse;
  status: (
    context: ExtensionContext,
    request: ForgeStatusRequest
  ) => Promise<ForgeStatusResponse> | ForgeStatusResponse;
  createPR: (
    context: ExtensionContext,
    request: ForgePRCreateRequest
  ) => Promise<ForgePRCreateResponse> | ForgePRCreateResponse;
}

function requestRecord(method: string, request: unknown): Record<string, unknown> {
  if (typeof request !== "object" || request === null || Array.isArray(request)) {
    throw new InvalidParamsError(`${method} params must be an object`);
  }
  return request as Record<string, unknown>;
}

function requiredString(method: string, request: Record<string, unknown>, field: string): string {
  const value = request[field];
  if (typeof value !== "string" || value.trim().length === 0) {
    throw new InvalidParamsError(`${method} requires a non-empty ${field}`);
  }
  return value;
}

function remoteURLs(method: string, request: Record<string, unknown>): string[] {
  const values = request.remote_urls;
  if (
    !Array.isArray(values) ||
    values.length === 0 ||
    values.some(value => typeof value !== "string" || value.trim().length === 0)
  ) {
    throw new InvalidParamsError(`${method} requires non-empty remote_urls`);
  }
  return values;
}

function capabilitiesRequest(request: unknown): ForgeCapabilitiesRequest {
  const record = requestRecord(FORGE_CAPABILITIES_METHOD, request);
  return { remote_urls: remoteURLs(FORGE_CAPABILITIES_METHOD, record) };
}

function statusRequest(request: unknown): ForgeStatusRequest {
  const record = requestRecord(FORGE_STATUS_METHOD, request);
  return {
    remote_urls: remoteURLs(FORGE_STATUS_METHOD, record),
    branch: requiredString(FORGE_STATUS_METHOD, record, "branch"),
  };
}

function createPRRequest(request: unknown): ForgePRCreateRequest {
  const record = requestRecord(FORGE_PR_CREATE_METHOD, request);
  const parsed: ForgePRCreateRequest = {
    remote_urls: remoteURLs(FORGE_PR_CREATE_METHOD, record),
    head: requiredString(FORGE_PR_CREATE_METHOD, record, "head"),
    base: requiredString(FORGE_PR_CREATE_METHOD, record, "base"),
    title: requiredString(FORGE_PR_CREATE_METHOD, record, "title"),
  };
  if (record.body !== undefined) {
    if (typeof record.body !== "string") {
      throw new InvalidParamsError(`${FORGE_PR_CREATE_METHOD} body must be a string`);
    }
    parsed.body = record.body;
  }
  if (record.draft !== undefined) {
    if (typeof record.draft !== "boolean") {
      throw new InvalidParamsError(`${FORGE_PR_CREATE_METHOD} draft must be a boolean`);
    }
    parsed.draft = record.draft;
  }
  return parsed;
}

export function registerForgeProvider(
  extension: Extension,
  handlers: ForgeProviderHandlers
): Extension {
  if (!extension) {
    throw new Error("extension is required");
  }
  if (
    typeof handlers?.capabilities !== "function" ||
    typeof handlers.status !== "function" ||
    typeof handlers.createPR !== "function"
  ) {
    throw new Error("forge provider handlers are required");
  }
  return extension[registerProvideSurface](FORGE_PROVIDER_CAPABILITY, [
 ...(handlers.searchPRs ? [{method:"forge/pr_search", handler:async (context:ExtensionContext, request:unknown) => {const record=requestRecord("forge/pr_search",request); return await handlers.searchPRs!(context,{remote_urls:remoteURLs("forge/pr_search",record),query:typeof record.query === "string" ? record.query : "",page:typeof record.page === "number" ? record.page : 1,limit:typeof record.limit === "number" ? record.limit : 30});}}] : []),
 ...(handlers.readPR ? [{method:"forge/pr_read", handler:async (context:ExtensionContext, request:unknown) => {const record=requestRecord("forge/pr_read",request); if(typeof record.number !== "number" || !Number.isInteger(record.number) || record.number < 1) throw new InvalidParamsError("positive PR number required"); return await handlers.readPR!(context,{remote_urls:remoteURLs("forge/pr_read",record),number:record.number});}}] : []),
    {
      method: FORGE_CAPABILITIES_METHOD,
      handler: async (context, request) =>
        await handlers.capabilities(context, capabilitiesRequest(request)),
    },
    {
      method: FORGE_STATUS_METHOD,
      handler: async (context, request) => await handlers.status(context, statusRequest(request)),
    },
    {
      method: FORGE_PR_CREATE_METHOD,
      handler: async (context, request) =>
        await handlers.createPR(context, createPRRequest(request)),
    },
  ]);
}
