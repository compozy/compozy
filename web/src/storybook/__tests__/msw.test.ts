import { HttpResponse } from "msw";
import { compozyApiMock } from "@/storybook/openapi-msw";
import { describe, expect, it } from "vitest";

import {
  composeStorybookHandlerGroup,
  storybookMswParameters,
  storybookSystemHandlerGroups,
  storybookSystemHandlers,
} from "../msw";

function handlerSignature(handler: { info: { method: unknown; path: unknown } }) {
  const method = String(handler.info.method);
  const path = String(handler.info.path).replace(/^\*/, "");
  return `${method} ${path}`;
}

describe("storybook msw helpers", () => {
  it("creates grouped story overrides without requiring untouched domains to be repeated", () => {
    const agentsOverride = [
      compozyApiMock.get("/api/agents", () => HttpResponse.json({ agents: [] })),
    ];
    const parameters = storybookMswParameters({ agent: agentsOverride });
    const mergedGroups = {
      ...storybookSystemHandlerGroups,
      ...parameters.msw.handlers,
    };

    expect(parameters).toEqual({
      msw: {
        handlers: {
          agent: composeStorybookHandlerGroup("agent", agentsOverride),
        },
      },
    });
    expect(mergedGroups.agent).toEqual(composeStorybookHandlerGroup("agent", agentsOverride));
    expect(mergedGroups.knowledge).toBe(storybookSystemHandlerGroups.knowledge);
    expect(mergedGroups.settings).toBe(storybookSystemHandlerGroups.settings);
    expect(mergedGroups.tasks).toBe(storybookSystemHandlerGroups.tasks);
  });

  it("preserves untouched handlers inside an overridden group while replacing matching endpoints", () => {
    const agentsOverride = [
      compozyApiMock.get("/api/agents", () => HttpResponse.json({ agents: [] })),
    ];
    const composedGroup = composeStorybookHandlerGroup("agent", agentsOverride);
    const signatures = composedGroup.map(handlerSignature);

    expect(composedGroup[0]).toBe(agentsOverride[0]);
    expect(signatures).toContain("GET /api/agents/catalog");
    expect(signatures.filter(signature => signature === "GET /api/agents")).toHaveLength(1);
  });

  it("Should keep concrete catalog routes ahead of param overrides that would shadow them", () => {
    const nameOverride = [
      compozyApiMock.get("/api/agents/{name}", () =>
        HttpResponse.json({
          agent: {
            name: "fraud-ops-agent",
            provider: "claude",
            prompt: "triage",
            origin: "global",
            definition_digest: "a".repeat(64),
          },
        })
      ),
    ];
    const composed = composeStorybookHandlerGroup("agent", nameOverride);
    const signatures = composed.map(handlerSignature);
    const catalogIdx = signatures.findIndex(signature => signature.includes("/api/agents/catalog"));
    const nameIdx = signatures.findIndex(
      signature =>
        /\/api\/agents\/(\{name\}|:name)$/.test(signature.replace(/^GET\s+\*?/, "GET ")) ||
        signature.endsWith("/api/agents/{name}") ||
        signature.endsWith("/api/agents/:name")
    );

    expect(catalogIdx).toBeGreaterThanOrEqual(0);
    expect(nameIdx).toBeGreaterThanOrEqual(0);
    expect(catalogIdx).toBeLessThan(nameIdx);
  });

  it("does not register duplicate local API method/path pairs after normalizing path params", () => {
    const signatures = storybookSystemHandlers
      .map(handlerSignature)
      .filter(signature => signature.includes(" /api/"))
      .map(signature => signature.replace(/:[^/]+/g, "{param}").replace(/\{[^/]+\}/g, "{param}"));

    expect(signatures).toHaveLength(new Set(signatures).size);
  });

  it("includes the route-owning vault handler group", () => {
    expect(storybookSystemHandlerGroups.vault.length).toBeGreaterThan(0);
  });

  it("includes the runtime handler group used by the shared app shell", () => {
    expect(storybookSystemHandlerGroups.runtime.length).toBeGreaterThan(0);
  });
});
