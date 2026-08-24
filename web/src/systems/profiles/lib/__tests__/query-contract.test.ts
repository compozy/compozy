// Suite: profile query identity contract
// Invariant: lens, target, and operation identities are canonical, and request names match their keys.
// Owning layer: web/src/systems/profiles/lib/query-keys.ts and query-options.ts.
// Boundary OUT: adapter decoding and React query lifecycle.
import { describe, expect, it, vi } from "vitest";

import {
  fetchArchivePlan,
  fetchDeletePlan,
  fetchProfile,
  fetchRenamePlan,
} from "../../adapters/profiles-api";
import { profileKeys } from "../query-keys";
import {
  archivePlanOptions,
  deletePlanOptions,
  profileDetailOptions,
  renamePlanOptions,
} from "../query-options";

vi.mock("../../adapters/profiles-api", () => ({
  fetchArchivePlan: vi.fn(),
  fetchDeletePlan: vi.fn(),
  fetchProfile: vi.fn(),
  fetchRenamePlan: vi.fn(),
}));

describe("profile query identities", () => {
  it("Should keep global and workspace selections in distinct cache slots", () => {
    expect(profileKeys.selection({ scope: "global" })).toEqual(["profiles", "selection", "global"]);
    expect(profileKeys.selection({ scope: "workspace", workspaceId: "workspace:alpha" })).toEqual([
      "profiles",
      "selection",
      "workspace:workspace:alpha",
    ]);
    expect(profileKeys.operations()).toEqual(["profiles", "operations"]);
  });

  it("Should normalize detail and lifecycle targets before keying or requesting", async () => {
    const signal = new AbortController().signal;
    vi.mocked(fetchProfile).mockResolvedValue({} as never);
    vi.mocked(fetchRenamePlan).mockResolvedValue({} as never);
    vi.mocked(fetchArchivePlan).mockResolvedValue({} as never);
    vi.mocked(fetchDeletePlan).mockResolvedValue({} as never);

    const detail = profileDetailOptions(" marketing ");
    const rename = renamePlanOptions(" marketing ", " growth ");
    const archive = archivePlanOptions(" marketing ");
    const remove = deletePlanOptions(" marketing ");

    expect(detail.queryKey).toEqual(["profiles", "detail", "marketing"]);
    expect(rename.queryKey).toEqual(["profiles", "plan", "rename", "marketing", "growth"]);
    expect(archive.queryKey).toEqual(["profiles", "plan", "archive", "marketing"]);
    expect(remove.queryKey).toEqual(["profiles", "plan", "delete", "marketing"]);

    await detail.queryFn?.({ signal } as never);
    await rename.queryFn?.({ signal } as never);
    await archive.queryFn?.({ signal } as never);
    await remove.queryFn?.({ signal } as never);

    expect(fetchProfile).toHaveBeenCalledExactlyOnceWith("marketing", signal);
    expect(fetchRenamePlan).toHaveBeenCalledExactlyOnceWith("marketing", "growth", signal);
    expect(fetchArchivePlan).toHaveBeenCalledExactlyOnceWith("marketing", signal);
    expect(fetchDeletePlan).toHaveBeenCalledExactlyOnceWith("marketing", signal);
  });

  it("Should disable blank detail and plan targets after normalization", () => {
    expect(profileDetailOptions(" ").enabled).toBe(false);
    expect(renamePlanOptions("marketing", " ").enabled).toBe(false);
    expect(archivePlanOptions(" ").enabled).toBe(false);
    expect(deletePlanOptions(" ").enabled).toBe(false);
  });
});
