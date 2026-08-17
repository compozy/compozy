import { describe, expect, it } from "vitest";

import { DeepLinkQueue, lastDeepLink, parseDeepLink, productNavigationURL } from "../deep-link";

// Invariant: only absolute, hostless product paths reach the renderer, and a pre-ready burst is last-wins exactly once.
// Owner: desktop deep-link boundary. Canonical suite: deep-link.test.ts.
describe("deep-link policy", () => {
  it("Should accept product paths and preserve their query", () => {
    expect(parseDeepLink("compozyos://open/sessions/abc?tab=logs")).toEqual({
      kind: "product",
      path: "/sessions/abc?tab=logs",
    });
  });

  it.each([
    "compozyos://open/http://evil.com",
    "compozyos://open//evil.com/x",
    "compozyos://open/../etc",
    "compozyos://open/%2e%2e/etc",
    "compozyos://open/..%5c..%5cetc",
    "https://example.com/sessions/abc",
  ])("Should reject hostile target %s to the default view", input => {
    expect(parseDeepLink(input)).toEqual({ kind: "default" });
  });

  it("Should hold the latest link until readiness and consume it once", () => {
    const queue = new DeepLinkQueue();
    queue.push("compozyos://open/sessions/first");
    queue.push("compozyos://open/sessions/last");
    expect(queue.take()).toBeNull();
    expect(queue.setReady()).toBe("/sessions/last");
    expect(queue.take()).toBeNull();
  });

  it("Should carry an explicit default-view intent without changing product paths", () => {
    expect(productNavigationURL("http://127.0.0.1:2123", "/")).toBe(
      "http://127.0.0.1:2123/?_compozy_desktop_default=1"
    );
    expect(productNavigationURL("http://127.0.0.1:2123", "/tasks?state=open")).toBe(
      "http://127.0.0.1:2123/tasks?state=open"
    );
  });

  it("Should extract only the last deep-link argument", () => {
    expect(
      lastDeepLink(["app", "compozyos://open/tasks/first", "--flag", "compozyos://open/tasks/last"])
    ).toBe("compozyos://open/tasks/last");
  });
});
