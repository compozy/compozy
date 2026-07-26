import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { expectFetchRequest, mockEmptyResponse, mockJsonResponse } from "@/test/fetch-test-utils";
import {
  listToolApprovalGrants,
  revokeToolApprovalGrant,
  setToolApprovalGrant,
  ToolApprovalGrantsApiError,
} from "@/systems/tool-approvals/adapters/tool-approvals-api";
import { toolApprovalGrantsResponseFixture } from "@/systems/tool-approvals/mocks/fixtures";

const WS = "ws_alpha";

beforeEach(() => {
  vi.stubGlobal("fetch", vi.fn());
});

afterEach(() => {
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
});

describe("listToolApprovalGrants", () => {
  it("Should scope the read to the workspace and return the daemon envelope", async () => {
    mockJsonResponse(toolApprovalGrantsResponseFixture);

    const result = await listToolApprovalGrants(WS);

    expect(result).toEqual(toolApprovalGrantsResponseFixture);
    await expectFetchRequest({ path: `/api/tool-approval-grants?workspace_id=${WS}` });
  });

  it("Should forward the abort signal to the GET", async () => {
    mockJsonResponse(toolApprovalGrantsResponseFixture);
    const controller = new AbortController();

    await listToolApprovalGrants(WS, controller.signal);

    await expectFetchRequest({
      path: `/api/tool-approval-grants?workspace_id=${WS}`,
      signal: controller.signal,
    });
  });

  it("Should throw a typed error on a non-2xx response", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(listToolApprovalGrants(WS)).rejects.toThrow(ToolApprovalGrantsApiError);
    await expect(listToolApprovalGrants(WS)).rejects.toThrow(
      "Failed to load remembered decisions: 500"
    );
  });
});

describe("setToolApprovalGrant", () => {
  it("Should PUT the exact wider request in one workspace and return the stored grant", async () => {
    const grant = { ...toolApprovalGrantsResponseFixture.grants[0]!, input_digest: undefined };
    mockJsonResponse({ grant });

    const result = await setToolApprovalGrant(WS, {
      tool_id: "agh__config_set",
      decision: "allow",
      scope: "agent",
      agent_name: "claude-code",
    });

    expect(result).toEqual(grant);
    await expectFetchRequest({
      method: "PUT",
      path: `/api/tool-approval-grants?workspace_id=${WS}`,
      body: {
        tool_id: "agh__config_set",
        decision: "allow",
        scope: "agent",
        agent_name: "claude-code",
      },
    });
  });

  it("Should throw a typed error on a rejected wider decision", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 400 }));

    await expect(
      setToolApprovalGrant(WS, {
        tool_id: "agh__config_set",
        decision: "allow",
        scope: "tool",
      })
    ).rejects.toThrow("Failed to set remembered decision: 400");
  });
});

describe("revokeToolApprovalGrant", () => {
  it("Should DELETE the grant by id scoped to the workspace", async () => {
    mockEmptyResponse({ status: 204 });

    await revokeToolApprovalGrant("grant_1", WS);

    await expectFetchRequest({
      method: "DELETE",
      path: `/api/tool-approval-grants/grant_1?workspace_id=${WS}`,
    });
  });

  it("Should map a 404 to a typed not-found error", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 404 }));

    await expect(revokeToolApprovalGrant("missing", WS)).rejects.toThrow(
      ToolApprovalGrantsApiError
    );
    await expect(revokeToolApprovalGrant("missing", WS)).rejects.toThrow(
      "Remembered decision not found"
    );
  });

  it("Should throw a typed error on other failures", async () => {
    vi.mocked(globalThis.fetch).mockResolvedValue(new Response(null, { status: 500 }));

    await expect(revokeToolApprovalGrant("grant_1", WS)).rejects.toThrow(
      "Failed to revoke remembered decision: 500"
    );
  });
});
