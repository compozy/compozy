import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { fetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { listAllModels, ModelCatalogApiError, refreshAllModels } from "../model-catalog-api";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe("listAllModels (aggregate catalog GET)", () => {
  it("Should GET the aggregate endpoint with the exact view + include_stale query", async () => {
    mockJsonResponse({ models: [] });

    await listAllModels({ view: "all", includeStale: true });

    const request = fetchRequest();
    const url = new URL(request.url);
    expect(request.method).toBe("GET");
    expect(url.pathname).toBe("/api/model-catalog/models");
    expect(url.searchParams.get("view")).toBe("all");
    expect(url.searchParams.get("include_stale")).toBe("true");
    // No per-provider fan-out: the aggregate carries no provider_id by default.
    expect(url.searchParams.get("provider_id")).toBeNull();
  });

  it("Should issue a bare aggregate GET when no query is supplied", async () => {
    mockJsonResponse({ models: [] });

    await listAllModels({});

    const url = new URL(fetchRequest().url);
    expect(url.pathname).toBe("/api/model-catalog/models");
    expect([...url.searchParams.keys()]).toEqual([]);
  });

  it("Should forward the AbortSignal to the aggregate request", async () => {
    mockJsonResponse({ models: [] });
    const controller = new AbortController();

    await listAllModels({ view: "all" }, controller.signal);

    const request = fetchRequest();
    expect(request.signal).toBeInstanceOf(AbortSignal);
    expect(request.signal.aborted).toBe(false);
    controller.abort();
    expect(request.signal.aborted).toBe(true);
  });

  it("Should return the aggregate payload on success", async () => {
    const payload = {
      models: [{ provider_id: "codex", model_id: "gpt-5.6" }],
    };
    mockJsonResponse(payload);

    const result = await listAllModels({ view: "all" });
    expect(result).toEqual(payload);
  });

  it("Should throw a typed ModelCatalogApiError carrying the response status on failure", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(
      new Response(JSON.stringify({ error: "forbidden" }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      })
    );

    const error = await listAllModels({ view: "all" }).catch(caught => caught);
    expect(error).toBeInstanceOf(ModelCatalogApiError);
    expect((error as ModelCatalogApiError).status).toBe(403);
  });
});

describe("refreshAllModels (aggregate catalog refresh)", () => {
  it("Should POST the aggregate refresh endpoint with the exact force body", async () => {
    mockJsonResponse({ models: [] });

    await refreshAllModels({ force: true, requestId: "req-1" });

    const request = fetchRequest();
    const url = new URL(request.url);
    expect(request.method).toBe("POST");
    expect(url.pathname).toBe("/api/model-catalog/models/refresh");
    expect(await request.clone().json()).toEqual({ force: true, request_id: "req-1" });
  });

  it("Should POST an empty body when no refresh input is supplied", async () => {
    mockJsonResponse({ models: [] });

    await refreshAllModels({});

    expect(await fetchRequest().clone().json()).toEqual({});
  });

  it("Should throw a typed ModelCatalogApiError carrying the response status on failure", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    const error = await refreshAllModels({}).catch(caught => caught);
    expect(error).toBeInstanceOf(ModelCatalogApiError);
    expect((error as ModelCatalogApiError).status).toBe(500);
  });
});
