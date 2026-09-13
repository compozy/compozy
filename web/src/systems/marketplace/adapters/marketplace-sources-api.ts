import { apiClient, apiRequestFailed, requireResponseData } from "@/lib/api-client";
import { marketplaceApiError, MarketplaceApiError } from "./marketplace-api-error";
import type {
  AddMarketplaceSourceRequest,
  UpdateMarketplaceSourceRequest,
  MarketplaceSourcesResponse,
  MarketplaceSourceResponse,
  MarketplaceSourcePreview,
} from "../types";

export class MarketplaceSourceApiError extends MarketplaceApiError {
  readonly suggestedName: string | undefined;
  readonly retainedBy: string[];
  readonly checked: string[];

  constructor(response: Response, body: unknown) {
    const error = marketplaceApiError("Failed to manage marketplace sources", response, body);
    super(error.message, error.status, error.diagnosticCode);
    this.name = "MarketplaceSourceApiError";
    const field = (key: string): unknown =>
      body !== null && typeof body === "object" ? Reflect.get(body, key) : undefined;
    const suggested = field("suggested_name");
    this.suggestedName = typeof suggested === "string" ? suggested : undefined;
    this.retainedBy = stringList(field("retained_by"));
    this.checked = stringList(field("checked"));
  }
}

function stringList(value: unknown): string[] {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string")
    : [];
}

export async function listMarketplaceSources(
  signal?: AbortSignal
): Promise<MarketplaceSourcesResponse> {
  const { data, error, response } = await apiClient.GET("/api/marketplace/sources", { signal });
  if (apiRequestFailed(response, error)) throw new MarketplaceSourceApiError(response, error);
  return requireResponseData(data, response, "Failed to load marketplace sources");
}

export async function previewMarketplaceSource(
  body: AddMarketplaceSourceRequest,
  signal?: AbortSignal
): Promise<MarketplaceSourcePreview> {
  const { data, error, response } = await apiClient.POST("/api/marketplace/sources", {
    body,
    params: { query: { dry_run: true } },
    signal,
  });
  if (apiRequestFailed(response, error)) throw new MarketplaceSourceApiError(response, error);
  const result = requireResponseData(data, response, "Failed to inspect marketplace source");
  if ("source" in result) throw new Error("Marketplace preview returned a registered source");
  return result;
}

export async function addMarketplaceSource(
  body: AddMarketplaceSourceRequest,
  signal?: AbortSignal
): Promise<MarketplaceSourceResponse> {
  const { data, error, response } = await apiClient.POST("/api/marketplace/sources", {
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) throw new MarketplaceSourceApiError(response, error);
  const result = requireResponseData(data, response, "Failed to add marketplace source");
  if (!("source" in result)) throw new Error("Marketplace registration returned a preview");
  return result;
}

export async function updateMarketplaceSource(
  name: string,
  body: UpdateMarketplaceSourceRequest,
  signal?: AbortSignal
): Promise<MarketplaceSourceResponse> {
  const { data, error, response } = await apiClient.PATCH("/api/marketplace/sources/{name}", {
    params: { path: { name } },
    body,
    signal,
  });
  if (apiRequestFailed(response, error)) throw new MarketplaceSourceApiError(response, error);
  return requireResponseData(data, response, "Failed to update marketplace source");
}

export async function removeMarketplaceSource(name: string, signal?: AbortSignal): Promise<void> {
  const { error, response } = await apiClient.DELETE("/api/marketplace/sources/{name}", {
    params: { path: { name } },
    signal,
  });
  if (apiRequestFailed(response, error)) throw new MarketplaceSourceApiError(response, error);
}

export async function refreshMarketplaceSource(
  name: string,
  signal?: AbortSignal
): Promise<MarketplaceSourceResponse> {
  const { data, error, response } = await apiClient.POST(
    "/api/marketplace/sources/{name}/refresh",
    {
      params: { path: { name } },
      signal,
    }
  );
  if (apiRequestFailed(response, error)) throw new MarketplaceSourceApiError(response, error);
  return requireResponseData(data, response, "Failed to refresh marketplace source");
}
