// Suite: router entry bridge over the real router
// Invariant: the coordinator's history writes add no state to their entries, so the router's own
// same-location check holds (a link to the URL shown commits nothing); the ledger learns each
// entry's identity from the key the router's history assigns: the coordinator's write and a
// redirect the router makes while loading it are its own, a link and Back are the operator's.
// Owning layer: the bridge between the coordinator and TanStack Router's history. It needs the
// real router: the same-location check and the redirect's replace live in router-core.
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  redirect,
} from "@tanstack/react-router";
import { describe, expect, it } from "vitest";

import { connectRouterEntries, RouterEntryBridge } from "../router-entry-bridge";
import { RoutingEntryLedger } from "../routing-entry-ledger";

function createHarness() {
  const rootRoute = createRootRoute();
  const tasksRoute = createRoute({ getParentRoute: () => rootRoute, path: "/tasks" });
  const settingsRoute = createRoute({ getParentRoute: () => rootRoute, path: "/settings" });
  // A loader-time canonicalization, like the session permalink adding its workspace binding.
  const legacyRoute = createRoute({
    getParentRoute: () => rootRoute,
    path: "/legacy",
    beforeLoad: () => {
      throw redirect({ to: "/settings", replace: true });
    },
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([tasksRoute, settingsRoute, legacyRoute]),
    history: createMemoryHistory({ initialEntries: ["/tasks"] }),
  });
  const entries = new RoutingEntryLedger();
  const bridge = new RouterEntryBridge(
    router.history,
    route => `${route.pathname}${router.options.stringifySearch(route.search)}`
  );
  const disconnect = connectRouterEntries(router, bridge, entries);
  const currentKey = () => router.history.location.state.__TSR_key ?? null;
  return { router, entries, bridge, disconnect, currentKey };
}

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

    bridge.port.navigate({ pathname: "/legacy", search: {} });
    const written = currentKey();
    await router.load();
    const redirected = currentKey();

    expect(router.history.location.pathname).toBe("/settings");
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
