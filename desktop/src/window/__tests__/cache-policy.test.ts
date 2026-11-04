import { describe, expect, it, vi } from "vitest";

import {
  configureDiskCache,
  HTTP_DISK_CACHE_MAX_BYTES,
  trimOversizedDiskCache,
} from "../cache-policy";

// Invariant: bound HTTP cache without clearing persistent user storage.
// Owner: Electron session boundary; canonical suite: desktop cache policy.
describe("desktop HTTP cache policy", () => {
  it("Should configure the Chromium disk cache limit before session startup", () => {
    const commandLine = { appendSwitch: vi.fn() };
    configureDiskCache(commandLine);
    expect(commandLine.appendSwitch).toHaveBeenCalledExactlyOnceWith(
      "disk-cache-size",
      "268435456"
    );
  });

  it.each([0, HTTP_DISK_CACHE_MAX_BYTES])(
    "Should retain an HTTP cache within the limit (%s)",
    async size => {
      const cache = { getCacheSize: vi.fn(async () => size), clearCache: vi.fn(async () => {}) };
      await trimOversizedDiskCache(cache);
      expect(cache.clearCache).not.toHaveBeenCalled();
    }
  );

  it("Should trim an oversized cache once and preserve user storage across launches", async () => {
    let bytes = 887 * 1024 * 1024;
    const cache = {
      getCacheSize: vi.fn(async () => bytes),
      clearCache: vi.fn(async () => {
        bytes = 0;
      }),
      clearStorageData: vi.fn(),
    };
    await trimOversizedDiskCache(cache);
    await trimOversizedDiskCache(cache);
    expect(cache.clearCache).toHaveBeenCalledTimes(1);
    expect(cache.clearStorageData).not.toHaveBeenCalled();
  });

  it("Should report cache I/O failures to the startup owner", async () => {
    const error = new Error("cache unavailable");
    await expect(
      trimOversizedDiskCache({
        getCacheSize: vi.fn(async () => {
          throw error;
        }),
        clearCache: vi.fn(),
      })
    ).rejects.toBe(error);
  });
});
