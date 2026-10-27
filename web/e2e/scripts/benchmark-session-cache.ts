// Synthetic cache CPU benchmark; it does not measure browser paint or network latency.
import { QueryClient } from "@tanstack/react-query";
import { setImmediate } from "node:timers";
import { createSessionReadReconciler } from "../../src/systems/session/hooks/session-catalog-reconciliation";
import { sessionKeys } from "../../src/systems/session/lib/query-keys";
import {
  invalidateSessionLiveQueries,
  invalidateSessionUsageQueries,
  invalidateSessionMonitorsQueries,
} from "../../src/systems/session/lib/session-query-invalidation";

for (const [name, invalidate] of Object.entries({
  surfaces: invalidateSessionLiveQueries,
  usage: invalidateSessionUsageQueries,
  catalog: undefined,
})) {
  const client = new QueryClient({
    defaultOptions: { queries: { gcTime: Infinity, retry: false } },
  });
  for (let i = 0; i < 128; i++) {
    const id = `session-${i}`;
    for (const key of [
      sessionKeys.detail("ws", id),
      sessionKeys.history("ws", id),
      sessionKeys.inputQueue("ws", id),
      sessionKeys.usage("ws", id),
      sessionKeys.usageTurns("ws", id),
      sessionKeys.transcript("ws", id),
      sessionKeys.goal("ws", id),
      sessionKeys.byId(id, "profile"),
    ])
      client.setQueryData(key, { state: "active" });
  }
  const catalogReads = createSessionReadReconciler(client);
  const samples: number[] = [];
  for (let sample = 0; sample < 13; sample++) {
    const started = performance.now();
    for (let i = 0; i < 64; i++) {
      const sessionId = `session-${i}`;
      if (invalidate) await invalidate(client, "ws", sessionId);
      else {
        catalogReads.wake(sessionId, {
          kind: "upserted",
          profile_id: "default",
          profile_name: "default",
          workspace_id: "ws",
          session_id: sessionId,
        });
        void invalidateSessionMonitorsQueries(client, "ws", sessionId);
      }
    }
    // No network requests in this fixture; drain all queued reconciliation promises.
    await new Promise<void>(resolve => setImmediate(resolve));
    const elapsed = performance.now() - started;
    if (sample >= 3) samples.push(elapsed);
  }
  samples.sort((a, b) => a - b);
  console.log(
    JSON.stringify({
      name,
      queries: client.getQueryCache().getAll().length,
      updates: 64,
      medianMs: (samples[4]! + samples[5]!) / 2,
      minMs: samples[0],
      maxMs: samples.at(-1),
    })
  );
  catalogReads.close();
  client.clear();
}
