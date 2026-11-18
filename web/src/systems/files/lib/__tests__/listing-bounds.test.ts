import { createEventRecorder } from "@/test/event-recorder";
import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it } from "vitest";

import { COLLAPSED_LISTING_LIMIT, pruneCollapsedListings } from "../listing-cache";
import { LISTING_CONCURRENCY, withListingSlot } from "../listing-slots";
import { filesKeys } from "../query-keys";

// Perf audit P-5. Invariant: no more than LISTING_CONCURRENCY folder listings run at once, a
// queued listing aborted before its turn never takes a slot, and every waiter eventually runs.
describe("withListingSlot", () => {
  it("Should cap concurrent listings and run the queued ones as slots free", async () => {
    let running = 0;
    let peak = 0;
    const listings = createEventRecorder<() => void>();
    const task = () =>
      new Promise<number>(resolve => {
        running += 1;
        peak = Math.max(peak, running);
        listings.record(() => {
          running -= 1;
          resolve(running);
        });
      });
    const aborted = new AbortController();
    const all = Array.from({ length: LISTING_CONCURRENCY + 4 }, () => withListingSlot(task));
    const cancelled = withListingSlot(task, aborted.signal);
    aborted.abort(new Error("collapsed"));
    await expect(cancelled).rejects.toThrow("collapsed");

    await listings.waitForCount(LISTING_CONCURRENCY);
    expect(peak).toBe(LISTING_CONCURRENCY);
    for (let index = 0; index < all.length; index += 1) {
      await listings.waitForCount(index + 1);
      listings.events[index]!();
    }
    await Promise.all(all);
    expect(peak).toBe(LISTING_CONCURRENCY);
  });
});

// Perf audit P-5. Invariant: a root keeps at most COLLAPSED_LISTING_LIMIT idle folder listings;
// the oldest go first, and shown folders and other roots are never dropped.
describe("pruneCollapsedListings", () => {
  it("Should drop the oldest idle listings beyond the limit", () => {
    const client = new QueryClient();
    const root = { kind: "workspace" as const, workspaceId: "ws_1" };
    const other = { kind: "workspace" as const, workspaceId: "ws_2" };
    const extra = 5;
    for (let index = 0; index < COLLAPSED_LISTING_LIMIT + extra; index += 1) {
      client.setQueryData(
        filesKeys.entries(root, `d${index}`, true),
        { entries: [] },
        {
          updatedAt: 1_000 + index,
        }
      );
    }
    client.setQueryData(filesKeys.entries(other, "d0", true), { entries: [] }, { updatedAt: 1 });

    const removed = pruneCollapsedListings(client, root);

    expect(removed.sort()).toEqual(["d0", "d1", "d2", "d3", "d4"]);
    expect(client.getQueryData(filesKeys.entries(root, "d5", true))).toBeDefined();
    expect(client.getQueryData(filesKeys.entries(other, "d0", true))).toBeDefined();
  });
});
