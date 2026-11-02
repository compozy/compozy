/**
 * Run from the repository root: bun web/e2e/scripts/benchmark-home-live.ts
 *
 * Measures actual QueryClient reads/cancellations for 100 task lifecycle events
 * while three active aggregate reads are pending. The deferred query functions
 * represent the I/O boundary. No browser/daemon, production credentials or network.
 * Compare the same harness on baseline and changed source; timings are synthetic
 * synchronous dispatch CPU, not user INP or endpoint latency.
 */
import { QueryClient, QueryObserver } from "@tanstack/react-query";
import { homeLiveRefreshLogic } from "../../src/systems/dashboard/hooks/home-live-refresh-store";

for (let sample = 0; sample < 5; sample++) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  let requests = 0,
    cancellations = 0,
    revision = 0;
  const pending: (() => void)[] = [];
  const keys = [["overview"], ["dashboard"], ["inbox"]];
  const observers = keys.map(queryKey => {
    const observer = new QueryObserver(queryClient, {
      queryKey,
      initialData: 0,
      staleTime: Infinity,
      queryFn: ({ signal }) => {
        requests++;
        const captured = revision;
        signal.addEventListener(
          "abort",
          () => {
            cancellations++;
          },
          { once: true }
        );
        return new Promise<number>(resolve => {
          pending.push(() => resolve(captured));
        });
      },
    });
    return observer.subscribe(() => {});
  });
  const store = homeLiveRefreshLogic.createStore();
  store.trigger.scopeActivated({ scope: "personal" });
  const invalidate = () =>
    Promise.all(keys.map(queryKey => queryClient.invalidateQueries({ queryKey })));
  const start = performance.now();
  for (revision = 1; revision <= 100; revision++) {
    store.trigger.activityReceived({
      at: Date.now(),
      scope: "personal",
      lifecycle: true,
      minimumIntervalMs: 60000,
      invalidateOverview: invalidate,
      invalidateTaskAggregates: invalidate,
    });
  }
  revision = 100;
  const burstMs = performance.now() - start;
  while (pending.length) {
    pending.splice(0).forEach(resolve => resolve());
    for (let i = 0; i < 10; i++) await Promise.resolve();
  }
  const latest = keys.map(key => queryClient.getQueryData(key));
  if (latest.some(value => value !== 100))
    throw new Error("Final lifecycle revision was not reconciled");
  console.log(JSON.stringify({ sample, requests, cancellations, burstMs, latest }));
  observers.forEach(unsubscribe => unsubscribe());
  queryClient.clear();
}
