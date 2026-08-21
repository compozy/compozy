// Suite: window-manager Settings adapter
// Invariant: the editor reads document + CAS revision from one authoritative snapshot, and
// resource discovery returns the complete unbounded collection exposed by the daemon.
// Owning layer: Settings HTTP adapter. Boundary OUT: React Query cache and editor presentation.
import { afterEach, describe, expect, it, vi } from "vitest";

import { windowManagerSnapshotFixture } from "@/systems/os/mocks";
import {
  windowManagerLayoutDocumentFixture,
  windowManagerLayoutResourceFixture,
} from "../../mocks/window-manager-fixtures";
import {
  parseWindowManagerLayoutDocument,
  parseWindowManagerLayoutResource,
} from "../../lib/window-manager-layout-schema";

import {
  applyWindowManagerLayout,
  deleteWindowManagerLayoutProfile,
  exportWindowManagerLayout,
  getWindowManagerLayoutState,
  listWindowManagerLayoutProfiles,
  previewWindowManagerLayout,
  putWindowManagerLayoutProfile,
  validateWindowManagerLayout,
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

    const state = await getWindowManagerLayoutState(
      windowManagerSnapshotFixture.workspace_id,
      "marketing"
    );

    expect(apiMocks.fetch).toHaveBeenCalledOnce();
    expect(apiMocks.fetch).toHaveBeenCalledWith(
      `/api/workspaces/${windowManagerSnapshotFixture.workspace_id}/window-manager?profile=marketing`,
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

    const profiles = await listWindowManagerLayoutProfiles("workspace-a", "marketing");

    expect(apiMocks.fetch).toHaveBeenCalledOnce();
    expect(apiMocks.fetch).toHaveBeenCalledWith(
      "/api/workspaces/workspace-a/window-manager/layout-profiles?profile=marketing",
      { signal: undefined }
    );
    expect(profiles.map(profile => profile.id)).toEqual([windowManagerLayoutResourceFixture.id]);
  });

  it("Should carry the profile through every layout read and mutation", async () => {
    const document = parseWindowManagerLayoutDocument(windowManagerLayoutDocumentFixture);
    const profile = parseWindowManagerLayoutResource({
      record: windowManagerLayoutResourceFixture,
    }).spec;
    apiMocks.fetch
      .mockResolvedValueOnce(jsonResponse(windowManagerLayoutDocumentFixture))
      .mockResolvedValueOnce(
        jsonResponse({ workspace_id: "workspace-a", valid: true, diagnostics: [] })
      )
      .mockResolvedValueOnce(
        jsonResponse({ snapshot: { revision: 8 }, changed: false, changes: {} })
      )
      .mockResolvedValueOnce(jsonResponse({ snapshot: { revision: 8 }, applied: true }))
      .mockResolvedValueOnce(jsonResponse({ record: windowManagerLayoutResourceFixture }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await exportWindowManagerLayout("workspace-a", " marketing ");
    await validateWindowManagerLayout("workspace-a", " marketing ", document);
    await previewWindowManagerLayout("workspace-a", " marketing ", 7, document);
    await applyWindowManagerLayout("workspace-a", " marketing ", 7, document);
    await putWindowManagerLayoutProfile(profile, "workspace", "workspace-a", " marketing ", 3);
    await deleteWindowManagerLayoutProfile("workspace-a", " marketing ", profile.id, 3);

    expect(apiMocks.fetch.mock.calls.map(call => call[0])).toEqual([
      "/api/workspaces/workspace-a/window-manager/layout?profile=marketing",
      "/api/workspaces/workspace-a/window-manager/layout/validate?profile=marketing",
      "/api/workspaces/workspace-a/window-manager/preview?profile=marketing",
      "/api/workspaces/workspace-a/window-manager/layout?profile=marketing",
      "/api/workspaces/workspace-a/window-manager/layout-profiles/launch-console?profile=marketing",
      "/api/workspaces/workspace-a/window-manager/layout-profiles/launch-console?profile=marketing",
    ]);
    expect(apiMocks.fetch.mock.calls.map(call => call[1]?.method ?? "GET")).toEqual([
      "GET",
      "POST",
      "POST",
      "PUT",
      "PUT",
      "DELETE",
    ]);
  });

  it("Should reject an empty layout profile before transport", async () => {
    await expect(exportWindowManagerLayout("workspace-a", "   ")).rejects.toMatchObject({
      message: "Profile is required.",
      status: 400,
    });
    expect(apiMocks.fetch).not.toHaveBeenCalled();
  });

  it("Should replace malformed response details with the adapter's safe JSON error", async () => {
    apiMocks.fetch.mockResolvedValueOnce(
      new Response("{not-json", {
        status: 502,
        headers: { "content-type": "application/json" },
      })
    );

    await expect(listWindowManagerLayoutProfiles("workspace-a", "marketing")).rejects.toMatchObject(
      {
        message: "CompozyOS returned invalid JSON.",
        status: 502,
      }
    );
  });
});
