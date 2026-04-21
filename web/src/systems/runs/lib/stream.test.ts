import { describe, expect, it } from "vitest";

import { buildRunStreamUrl } from "./stream";

describe("buildRunStreamUrl", () => {
  it("Should build a canonical stream URL without a cursor", () => {
    expect(buildRunStreamUrl({ runId: "r1", baseUrl: "http://localhost:1234" })).toBe(
      "http://localhost:1234/api/runs/r1/stream"
    );
  });

  it("Should attach a cursor query parameter when provided", () => {
    const url = buildRunStreamUrl({
      runId: "r1",
      baseUrl: "http://localhost:1234/",
      lastEventId: "2026-01-01T00:00:00Z|00000000000000000001",
    });
    expect(url).toBe(
      "http://localhost:1234/api/runs/r1/stream?cursor=2026-01-01T00%3A00%3A00Z%7C00000000000000000001"
    );
  });

  it("Should encode run ids for URL safety", () => {
    const url = buildRunStreamUrl({ runId: "run with space", baseUrl: "http://x" });
    expect(url).toBe("http://x/api/runs/run%20with%20space/stream");
  });
});
