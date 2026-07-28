// Suite: window-manager Settings adapter
// Invariant: the editor reads document + CAS revision from one authoritative snapshot, and
// resource discovery returns the complete unbounded collection exposed by the daemon.
// Owning layer: Settings HTTP adapter. Boundary OUT: React Query cache and editor presentation.
import { afterEach, describe, expect, it, vi } from "vitest";

import { windowManagerSnapshotFixture } from "@/systems/os/mocks";
import { windowManagerLayoutResourceFixture } from "../../mocks/window-manager-fixtures";

import {
  getWindowManagerLayoutState,
  listWindowManagerLayoutProfiles,
} from "../window-manager-layouts-api";

const apiMocks = vi.hoisted(() => ({
  fetch: vi.fn(),
}));

vi.mock("@/lib/api-client", () => ({
  apiBaseUrl: "",
  runtimeFetch: apiMocks.fetch,
}));

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

afterEach(() => {
  vi.clearAllMocks();
});

describe("window-manager layouts API", () => {
  it("Should derive the exported document and revision from one snapshot response", async () => {
    apiMocks.fetch.mockResolvedValueOnce(jsonResponse(windowManagerSnapshotFixture));

    const state = await getWindowManagerLayoutState(windowManagerSnapshotFixture.workspace_id);

    expect(apiMocks.fetch).toHaveBeenCalledOnce();
    expect(apiMocks.fetch).toHaveBeenCalledWith(
      `/api/workspaces/${windowManagerSnapshotFixture.workspace_id}/window-manager`,
      { signal: undefined }
    );
    expect(state).toMatchObject({
      revision: windowManagerSnapshotFixture.revision,
      document: {
        version: windowManagerSnapshotFixture.version,
        workspaceId: windowManagerSnapshotFixture.workspace_id,
      },
    });
    expect(state.document.desktops).toHaveLength(windowManagerSnapshotFixture.desktops.length);
    expect(Object.keys(state.document.windows)).toEqual(
      Object.keys(windowManagerSnapshotFixture.windows)
    );
  });

  it("Should request every visible layout profile without a silent client-side cap", async () => {
    apiMocks.fetch.mockResolvedValueOnce(
      jsonResponse({ records: [windowManagerLayoutResourceFixture] })
    );

    const profiles = await listWindowManagerLayoutProfiles("workspace-a");

    expect(apiMocks.fetch).toHaveBeenCalledOnce();
    expect(apiMocks.fetch).toHaveBeenCalledWith(
      "/api/workspaces/workspace-a/window-manager/layout-profiles",
      { signal: undefined }
    );
    expect(profiles.map(profile => profile.id)).toEqual([windowManagerLayoutResourceFixture.id]);
  });
});
