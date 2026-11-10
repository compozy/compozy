import { readFileSync, readdirSync } from "node:fs";
import { resolve } from "node:path";

import { render, screen } from "@testing-library/react";
import { createElement } from "react";
import { describe, expect, it } from "vitest";

import { SdkResourceBody } from "@/components/sdk/sdk-resource-body";
import { siteRoot } from "@/lib/content-test-utils";
import {
  buildSdkReference,
  renderResourceStub,
  renderSdkMeta,
  type OpenAPIDocument,
  type SdkReferenceInputs,
} from "@/lib/sdk-reference-build";
import { returnsOf, signatureOf, type SdkLang, type SdkReference } from "@/lib/sdk-reference-model";
import type { SDKSampleManifest } from "@/lib/sdk-samples";

const repoRoot = resolve(siteRoot, "../..");
const read = (...parts: string[]) => readFileSync(resolve(repoRoot, ...parts), "utf8");

function inputs(): SdkReferenceInputs {
  const goDir = resolve(repoRoot, "sdk/go/client");
  const goSource = readdirSync(goDir)
    .filter(file => file.endsWith(".go") && !file.endsWith("_test.go"))
    .sort()
    .map(file => readFileSync(resolve(goDir, file), "utf8"))
    .join("\n");
  return {
    openapi: JSON.parse(read("openapi/compozy.json")) as OpenAPIDocument,
    samples: JSON.parse(read("openapi/sdk-samples.json")) as SDKSampleManifest,
    goSource,
    tsTypes: read("sdk/client/src/generated/sdk/types.gen.ts"),
    tsSseTypes: read("sdk/client/src/generated/sse/types.gen.ts"),
  };
}

const source = inputs();
const reference = buildSdkReference(source);

function find(reference: SdkReference, id: string) {
  for (const resource of reference.resources) {
    const method = resource.ops.find(candidate => candidate.id === id);
    if (method) return { resource, method };
  }
  throw new Error(`no SDK method ${id}`);
}

describe("SDK reference generator", () => {
  it("writes the same model, stubs and meta from the same inputs", () => {
    const again = buildSdkReference(inputs());
    expect(again).toEqual(reference);
    expect(again.resources.map(renderResourceStub)).toEqual(
      reference.resources.map(renderResourceStub)
    );
    expect(renderSdkMeta(again)).toBe(renderSdkMeta(reference));
  });

  it("puts every sampled operation on exactly one resource page", () => {
    const placed = reference.resources.flatMap(resource => resource.ops.map(method => method.id));
    expect(placed.toSorted()).toEqual(Object.keys(source.samples.operations).toSorted());
    expect(new Set(placed).size).toBe(placed.length);
    for (const resource of reference.resources) {
      expect(renderResourceStub(resource)).toContain(`<SdkResource id="${resource.id}" />`);
      for (const method of resource.ops) {
        expect(source.samples.operations[method.id]?.resource).toBe(resource.id);
      }
    }
    expect(reference.stats.operations).toBe(placed.length);
    expect(reference.stats.resources).toBe(reference.resources.length);
  });

  it("orders resources by method count, then id, behind the client pages", () => {
    const counts = reference.resources.map(resource => resource.ops.length);
    expect(counts).toEqual(counts.toSorted((left, right) => right - left));
    const meta = JSON.parse(renderSdkMeta(reference)) as { pages: string[] };
    expect(meta.pages.slice(0, 7)).toEqual([
      "index",
      "---Client---",
      "client",
      "prompt",
      "events",
      "errors",
      "---Resources---",
    ]);
    expect(meta.pages.slice(7)).toEqual(reference.resources.map(resource => resource.slug));
  });

  it("labels stream and prompt methods in the method index", () => {
    const { resource } = find(reference, "streamSession");
    render(createElement(SdkResourceBody, { resource }));
    const label = (id: string) => {
      const link = screen.getAllByRole("link").find(item => item.getAttribute("href") === `#${id}`);
      if (!link) throw new Error(`no index entry for ${id}`);
      return link.querySelector("span")?.textContent;
    };
    expect(label("streamSession")).toBe("sse");
    expect(label("sendSessionPrompt")).toBe("prompt");
    expect(label("listSessions")).toBeUndefined();
  });
});

describe("SDK call shapes", () => {
  const cases: [string, SdkLang, string | null, string | null][] = [
    [
      "listSessions",
      "ts",
      "compozy.sessions.listSessions({ query })",
      "Promise<SessionCatalogResponse>",
    ],
    ["listSessions", "go", "c.ListSessions(ctx, ListSessionsParams)", "*SessionCatalogResponse"],
    [
      "createSession",
      "go",
      "c.CreateSession(ctx, CreateSessionParams, CreateSessionRequest)",
      "*SessionResponse",
    ],
    [
      "streamSession",
      "ts",
      'compozy.events("streamSession", { path, query })',
      'AsyncIterable<StreamEvent<SseFrame<"streamSession">>>',
    ],
    [
      "streamSession",
      "go",
      "c.StreamSession(ctx, StreamSessionParams)",
      "*Stream[StreamSessionFrame]",
    ],
    ["sendSessionPrompt", "ts", "compozy.prompt(input)", "Promise<PromptReply>"],
    ["sendSessionPrompt", "go", "c.Prompt(ctx, PromptInput)", "*PromptStream"],
    ["streamTerminal", "ts", null, null],
    ["streamTerminal", "go", null, null],
  ];

  it.each(cases)("%s in %s", (id, lang, signature, returns) => {
    const { resource, method } = find(reference, id);
    expect(signatureOf(resource, method, lang)).toBe(signature);
    expect(returnsOf(method, lang)).toBe(returns);
  });

  it("matches the calls the shipped samples make for every JSON method", () => {
    for (const resource of reference.resources) {
      for (const method of resource.ops) {
        if (method.kind !== "json") continue;
        if (method.samples.ts) {
          expect(method.samples.ts).toContain(`compozy.${resource.id}.${method.id}(`);
        }
        if (method.samples.go) expect(method.samples.go).toContain(`c.${method.go?.name}(`);
      }
    }
  });
});
