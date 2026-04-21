import { afterEach, describe, expect, it } from "vitest";

import { installFetchStub, matchPath } from "@/test/utils";

import { getWorkflowSpec } from "./spec-api";

describe("spec api adapter", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should GET the workflow spec with the workspace header", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/alpha/spec"),
        status: 200,
        body: {
          spec: {
            workspace: {
              id: "ws-1",
              name: "one",
              root_dir: "/tmp/one",
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
            workflow: { id: "wf-1", slug: "alpha", workspace_id: "ws-1" },
            prd: {
              id: "prd",
              kind: "prd",
              title: "PRD: alpha",
              updated_at: "2026-01-01T00:00:00Z",
              markdown: "# PRD body",
            },
            techspec: {
              id: "techspec",
              kind: "techspec",
              title: "TechSpec: alpha",
              updated_at: "2026-01-01T00:00:00Z",
              markdown: "# TechSpec body",
            },
            adrs: [
              {
                id: "adr-001",
                kind: "adr",
                title: "ADR-001",
                updated_at: "2026-01-01T00:00:00Z",
                markdown: "ADR body",
              },
            ],
          },
        },
      },
    ]);
    restore = stub.restore;
    const result = await getWorkflowSpec({ workspaceId: "ws-1", slug: "alpha" });
    expect(result.prd?.title).toBe("PRD: alpha");
    expect(result.techspec?.markdown).toContain("TechSpec body");
    expect(result.adrs?.[0]?.id).toBe("adr-001");
    expect(stub.calls[0]?.headers["x-compozy-workspace-id"]).toBe("ws-1");
  });

  it("Should surface transport errors when the spec is missing", async () => {
    const stub = installFetchStub([
      {
        matcher: matchPath("/api/tasks/missing/spec"),
        status: 404,
        body: { code: "document_missing", message: "spec missing", request_id: "r" },
      },
    ]);
    restore = stub.restore;
    await expect(getWorkflowSpec({ workspaceId: "ws-1", slug: "missing" })).rejects.toThrow(
      /spec missing/
    );
  });
});
