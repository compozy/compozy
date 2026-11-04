import type { App, Session } from "electron";

// Holds several immutable UI bundles without retaining an unbounded API response cache.
export const HTTP_DISK_CACHE_MAX_BYTES = 256 * 1024 * 1024;

export function configureDiskCache(commandLine: Pick<App["commandLine"], "appendSwitch">): void {
  commandLine.appendSwitch("disk-cache-size", String(HTTP_DISK_CACHE_MAX_BYTES));
}

export async function trimOversizedDiskCache(
  cache: Pick<Session, "getCacheSize" | "clearCache">
): Promise<void> {
  if ((await cache.getCacheSize()) > HTTP_DISK_CACHE_MAX_BYTES) await cache.clearCache();
}
