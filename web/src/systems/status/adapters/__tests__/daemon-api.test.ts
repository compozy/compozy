import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockJsonResponse } from "@/test/fetch-test-utils";

import { statusFixture } from "../../mocks/fixtures";
import { fetchStatus } from "../daemon-api";

describe("fetchStatus", () => {
  beforeEach(() => {
    vi.stubGlobal("fetch", vi.fn());
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("Should return the complete status envelope on success", async () => {
    mockJsonResponse(statusFixture);

    const result = await fetchStatus();

    expect(result).toEqual(statusFixture);
    expect(result.daemon.user_home_dir).toBe("/Users/pedro");
    expect(result.health).toEqual(statusFixture.health);
    await expectFetchRequest({ path: "/api/status" });
  });

  it("Should pass the abort signal to fetch", async () => {
    mockJsonResponse(statusFixture);

    const controller = new AbortController();
    await fetchStatus(controller.signal);

    await expectFetchRequest({
      path: "/api/status",
      signal: controller.signal,
    });
  });

  it("Should throw the runtime status error for a non-success response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(fetchStatus()).rejects.toThrow("Runtime status check failed: 500");
  });

  it("Should preserve a rejected fetch error", async () => {
    vi.mocked(globalThis.fetch).mockRejectedValue(new TypeError("Failed to fetch"));

    await expect(fetchStatus()).rejects.toThrow("Failed to fetch");
  });
});
