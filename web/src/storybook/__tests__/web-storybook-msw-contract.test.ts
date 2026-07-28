import type { HttpHandler } from "msw";
import { describe, expect, it } from "vitest";

const { storybookSystemHandlerGroups, storybookSystemHandlers } =
  await import("../../../.storybook/preview");
const { flattenStorybookHandlerGroups } = await import("../msw");

function handlerSignature(handler: HttpHandler) {
  const method = String(handler.info.method);
  const path = String(handler.info.path).replace(/^\*/, "");
  return `${method} ${path}`;
}

function normalizedHandlerSignature(handler: HttpHandler) {
  return handlerSignature(handler)
    .replace(/:[^/]+/g, "{param}")
    .replace(/\{[^/]+\}/g, "{param}");
}

describe("web Storybook MSW contract", () => {
  it("exposes default Storybook system handlers", () => {
    expect(storybookSystemHandlers.length).toBeGreaterThan(0);
    expect(flattenStorybookHandlerGroups(storybookSystemHandlerGroups).length).toBe(
      storybookSystemHandlers.length
    );
  });

  it("does not register duplicate method/path handler pairs across the combined system set", () => {
    const signatures = storybookSystemHandlers.map(normalizedHandlerSignature);
    const uniqueSignatures = new Set(signatures);

    expect(uniqueSignatures.size).toBe(signatures.length);
  });

  it("registers the onboarding status handler required by app route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/onboarding");
  });

  it("registers the runtime log stream handler required by nav count stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/logs/stream");
  });

  it("registers the provider catalog and directory browser handlers required by onboarding route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/providers");
    expect(signatures).toContain("GET /api/fs/browse");
  });

  it("registers the bridge health stream handler required by bridge route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/bridges/health/stream");
  });

  it("registers the marketplace search handler required by marketplace route stories", () => {
    const signatures = storybookSystemHandlers.map(handlerSignature);

    expect(signatures).toContain("GET /api/marketplace/search");
  });

  it("registers the loop run event stream handler required by loop run route stories", () => {
    const signatures = storybookSystemHandlers.map(normalizedHandlerSignature);

    expect(signatures).toContain("GET /api/workspaces/{param}/loop-runs/{param}/events");
  });
});
