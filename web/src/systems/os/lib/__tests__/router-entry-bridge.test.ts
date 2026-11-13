// Suite: router entry bridge over the real router
// Invariant: the coordinator's history writes add no state to their entries, so the router's own
// same-location check holds (a link to the URL shown commits nothing); the ledger learns each
// entry's identity from the key the router's history assigns: the entry the coordinator's own
// call committed and a same-path redirect the router makes while loading it are its own; a link,
// Back, a replace to another path and a write a navigation blocker held are not.
// Owning layer: the bridge between the coordinator and TanStack Router's history. It needs the
// real router and history: the same-location check, the redirect's replace and the blocker's
// deferral live in router-core and the history.
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";
import { describe, expect, it, vi } from "vitest";

import { connectRouterEntries, RouterEntryBridge } from "../router-entry-bridge";
import { RoutingEntryLedger } from "../routing-entry-ledger";

function createHarness() {
  const rootRoute = createRootRoute();
  const tasksRoute = createRoute({ getParentRoute: () => rootRoute, path: "/tasks" });
  const settingsRoute = createRoute({ getParentRoute: () => rootRoute, path: "/settings" });
  // A loader-time canonicalization, like the session permalink adding its workspace binding.
  const boundRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/bound",
    beforeLoad: ({ location }) => {
      if (!("workspace" in location.search)) {
        throw redirect({ href: "/bound?workspace=w", replace: true });
      }
    },
  });
  // A loader that only finishes when the test releases it.
  let releaseSlow: () => void = () => undefined;
  const slowRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/slow",
    loader: () =>
      new Promise<void>(resolve => {
        releaseSlow = resolve;
      }),
  });
  // A loader redirect to another path.
  const legacyRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/legacy",
    beforeLoad: () => {
      throw redirect({ to: "/settings", replace: true });
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([
      tasksRoute,
      settingsRoute,
      boundRoute,
      slowRoute,
      legacyRoute,
    ]),
    history: createMemoryHistory({ initialEntries: ["/tasks"] }),
  });
  const entries = new RoutingEntryLedger();
  const bridge = new RouterEntryBridge(
    router.history,
    route => `${route.pathname}${router.options.stringifySearch(route.search)}`
  );
  // As the mounted app's Transitioner does: a history with subscribers no longer loads on its own
  // (router-core `commitLocation`), so the router loads each history change from here.
  const stopLoading = router.history.subscribe(() => {
    void router.load();
  });
  const stopEntries = connectRouterEntries(router, bridge, entries);
  const disconnect = () => {
    stopEntries();
    stopLoading();
  };
  const currentKey = () => router.history.location.state.__TSR_key ?? null;
  return {
    router,
    entries,
    bridge,
    disconnect,
    currentKey,
    releaseSlow: () => releaseSlow(),
  };
}

/**
 * A navigation blocker answering one request at a time, as the unsaved-editor guard's `useBlocker`
 * resolver does: `answer(true)` keeps editing (the navigation is dropped), `answer(false)`
 * discards the changes and lets it through.
 */
function holdNavigations(router: ReturnType<typeof createHarness>["router"]) {
  let resolveHeld: ((blocked: boolean) => void) | null = null;
  const release = router.history.block({
    blockerFn: () =>
      new Promise<boolean>(resolve => {
        resolveHeld = resolve;
      }),
    enableBeforeUnload: false,
  });
  return {
    held: () => resolveHeld !== null,
    answer: (blocked: boolean) => {
      const resolve = resolveHeld;
      resolveHeld = null;
      resolve?.(blocked);
    },
    release,
  };
}

const settle = () => new Promise(resolve => setTimeout(resolve, 0));

describe("RouterEntryBridge", () => {
  it("Should commit nothing when a link targets the URL a coordinator write shows [R10-02]", async () => {
    const { router, bridge, disconnect, currentKey } = createHarness();
    await router.load();
    bridge.port.navigate({ pathname: "/settings", search: {} });
    await router.load();
    const written = currentKey();
    const index = router.history.location.state.__TSR_index;
    const length = router.history.length;

    await router.navigate({ to: "/settings" });

    expect(router.history.location.pathname).toBe("/settings");
    expect(router.history.location.state.__TSR_index).toBe(index);
    expect(router.history.length).toBe(length);
    expect(currentKey()).toBe(written);
    disconnect();
  });

  it("Should own every entry of a write the router redirects and none of a link or Back [AS-F-17]", async () => {
    const { router, entries, bridge, disconnect, currentKey } = createHarness();
    await router.load();
    const start = currentKey();

    bridge.port.navigate({ pathname: "/bound", search: {} });
    const written = currentKey();
    await router.load();
    const redirected = currentKey();

    expect(router.history.location.pathname).toBe("/bound");
    expect(router.history.location.search).toContain("workspace");
    expect(redirected).not.toBe(written);
    expect(entries.isEcho(written)).toBe(true);
    expect(entries.isEcho(redirected)).toBe(true);
    expect(entries.isEcho(start)).toBe(false);

    await router.navigate({ to: "/tasks" });
    const linked = currentKey();
    expect(entries.isEcho(linked)).toBe(false);

    router.history.back();
    expect(currentKey()).toBe(redirected);
    // Back landed on an entry the coordinator wrote: the traversal is the operator's.
    expect(entries.isEcho(redirected)).toBe(false);
    disconnect();
  });

  it("Should read a loader redirect of a write to another path as the router's own navigation", async () => {
    const { router, entries, bridge, disconnect, currentKey } = createHarness();
    await router.load();

    bridge.port.navigate({ pathname: "/legacy", search: {} });
    const written = currentKey();
    await router.load();

    // History cannot tell this redirect from an app's replace to another path: both read as intent.
    expect(router.history.location.pathname).toBe("/settings");
    expect(entries.isEcho(written)).toBe(true);
    expect(entries.isEcho(currentKey())).toBe(false);
    disconnect();
  });

  it("Should read an app's replace to another path during an owned load as intent [AS-F-17]", async () => {
    const { router, entries, bridge, disconnect, currentKey, releaseSlow } = createHarness();
    await router.load();
    bridge.port.navigate({ pathname: "/slow", search: {} });
    const written = currentKey();

    // The written entry is still loading: its loader has not returned.
    void router.navigate({ to: "/tasks", replace: true });
    await vi.waitFor(() => expect(router.history.location.pathname).toBe("/tasks"));

    expect(currentKey()).not.toBe(written);
    expect(entries.isEcho(written)).toBe(true);
    expect(entries.isEcho(currentKey())).toBe(false);
    releaseSlow();
    disconnect();
  });

  it("Should read an operator link as intent after a blocker cancelled a write to the same URL [AS-F-17]", async () => {
    const { router, entries, bridge, disconnect, currentKey } = createHarness();
    await router.load();
    const start = currentKey();
    const blocker = holdNavigations(router);

    bridge.port.navigate({ pathname: "/settings", search: {} });
    await settle();
    expect(blocker.held()).toBe(true);
    // Keep editing: the write never commits.
    blocker.answer(true);
    await settle();
    expect(currentKey()).toBe(start);
    blocker.release();

    await router.navigate({ to: "/settings" });

    expect(router.history.location.pathname).toBe("/settings");
    expect(entries.isEcho(currentKey())).toBe(false);
    disconnect();
  });

  it("Should read a write a blocker let through later as intent, in one entry [AS-F-17]", async () => {
    const { router, entries, bridge, disconnect, currentKey } = createHarness();
    await router.load();
    const length = router.history.length;
    const blocker = holdNavigations(router);

    bridge.port.navigate({ pathname: "/settings", search: {} });
    await settle();
    expect(blocker.held()).toBe(true);
    // Discard changes: the write lands outside the bridge's call.
    blocker.answer(false);
    await vi.waitFor(() => expect(router.history.location.pathname).toBe("/settings"));

    expect(router.history.length).toBe(length + 1);
    expect(entries.isEcho(currentKey())).toBe(false);
    blocker.release();
    disconnect();
  });

  it("Should read a held write that lands after another navigation as intent [AS-F-17]", async () => {
    const { router, entries, bridge, disconnect, currentKey } = createHarness();
    await router.load();
    const blocker = holdNavigations(router);

    bridge.port.navigate({ pathname: "/settings", search: {} });
    await settle();
    expect(blocker.held()).toBe(true);
    // Another navigation the blocker does not hold overtakes the held write.
    await router.navigate({ to: "/bound", ignoreBlocker: true });
    const overtaking = currentKey();
    expect(entries.isEcho(overtaking)).toBe(false);

    blocker.answer(false);
    await vi.waitFor(() => expect(router.history.location.pathname).toBe("/settings"));

    expect(currentKey()).not.toBe(overtaking);
    expect(entries.isEcho(currentKey())).toBe(false);
    blocker.release();
    disconnect();
  });

  it("Should leave a replace after the written entry loaded to whoever made it", async () => {
    const { router, entries, bridge, disconnect, currentKey } = createHarness();
    await router.load();
    bridge.port.navigate({ pathname: "/settings", search: {} });
    await router.load();

    await router.navigate({ to: "/tasks", replace: true });

    expect(entries.isEcho(currentKey())).toBe(false);
    disconnect();
  });
});
