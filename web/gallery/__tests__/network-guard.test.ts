// @vitest-environment node
import { mkdirSync, mkdtempSync, realpathSync, rmSync, writeFileSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { build } from "vite";
import { afterEach, describe, expect, it } from "vitest";

import { forbiddenChain, isPrototypeModule, networkModuleMatcher } from "../node/network-graph";
import { networkGuard } from "../node/plugins/network-guard";

const roots: string[] = [];

afterEach(() => {
  for (const root of roots.splice(0)) rmSync(root, { recursive: true, force: true });
});

function web(files: Record<string, string>) {
  const root = realpathSync(mkdtempSync(join(tmpdir(), "compozy-guard-")));
  roots.push(root);
  for (const [file, body] of Object.entries(files)) {
    const path = join(root, file);
    mkdirSync(join(path, ".."), { recursive: true });
    writeFileSync(path, body);
  }
  return root;
}

async function buildPrototype(root: string, entry: string) {
  await build({
    root,
    configFile: false,
    logLevel: "silent",
    build: { write: false, lib: { entry: join(root, entry), formats: ["es"], fileName: "out" } },
    plugins: [networkGuard(root)],
  });
}

const client = { "src/lib/api-client.ts": "export const api = { get: () => fetch('/api') };" };

describe("prototype network guard", () => {
  it("matches the daemon network layer and prototype modules", () => {
    const forbidden = networkModuleMatcher("/repo/web");
    expect(forbidden("/repo/web/src/lib/api-client.ts")).toBe(true);
    expect(forbidden("/repo/web/src/lib/ticketed-web-socket.ts")).toBe(true);
    expect(forbidden("/repo/web/src/lib/gateway-stream-auth.ts")).toBe(true);
    expect(forbidden("/repo/node_modules/@tanstack/react-query/build/modern/index.js")).toBe(true);
    expect(forbidden("/repo/web/src/lib/utils.ts")).toBe(false);
    expect(isPrototypeModule("/repo/web/src/systems/a/card.proto.tsx")).toBe(true);
    expect(isPrototypeModule("/repo/web/src/drafts/board/view.tsx")).toBe(true);
    expect(isPrototypeModule("/repo/web/src/systems/a/card.tsx")).toBe(false);
  });

  it("returns the first import chain to a forbidden module", () => {
    const graph = new Map([
      ["proto", new Set(["view", "fixture"])],
      ["view", new Set(["hook"])],
      ["hook", new Set(["client"])],
    ]);
    expect(forbiddenChain("proto", graph, id => id === "client")).toEqual([
      "proto",
      "view",
      "hook",
      "client",
    ]);
    expect(forbiddenChain("fixture", graph, id => id === "client")).toBeUndefined();
  });

  it("fails the build when a prototype reaches the API client through a view", async () => {
    const root = web({
      ...client,
      "src/systems/a/view.ts":
        "import { api } from '../../lib/api-client';\nexport const view = api;",
      "src/systems/a/card.proto.ts": "import { view } from './view';\nexport default view;",
    });
    await expect(buildPrototype(root, "src/systems/a/card.proto.ts")).rejects.toThrow(
      "api_client: src/systems/a/card.proto.ts reaches the daemon network layer: src/systems/a/card.proto.ts → src/systems/a/view.ts → src/lib/api-client.ts"
    );
  });

  it("allows type-only imports of the network layer", async () => {
    const root = web({
      ...client,
      "src/systems/a/card.proto.ts":
        "import type { api } from '../../lib/api-client';\nexport default {} as typeof api;",
    });
    await expect(buildPrototype(root, "src/systems/a/card.proto.ts")).resolves.toBeUndefined();
  });
});
