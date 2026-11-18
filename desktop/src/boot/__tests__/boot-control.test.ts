import { describe, expect, it, vi } from "vitest";

import { createBootControl, releaseNotesURL } from "../boot-control";

function control(targetVersion: string | undefined, configFailure = "") {
  const openExternal = vi.fn(async (_url: string) => {});
  const forward = vi.fn(async () => ({ accepted: true }));
  const handle = createBootControl({
    openLogs: async () => "",
    openConfig: async () => configFailure,
    openExternal,
    targetVersion: () => targetVersion,
    quit: () => {},
    forward,
  });
  return { handle, openExternal, forward };
}

// Invariant: boot page actions run only the shell-local action they name, and release notes open
// only the GitHub release of the version this boot moves to.
// Owner: desktop boot control boundary. Canonical suite: boot-control.test.ts.
describe("boot control", () => {
  it("Should open the release notes of the target version", async () => {
    const { handle, openExternal } = control("0.6.0");
    await expect(handle("open_release_notes", {})).resolves.toEqual({ opened: true });
    expect(openExternal).toHaveBeenCalledWith(
      "https://github.com/compozy/compozy/releases/tag/v0.6.0"
    );
  });

  it.each([undefined, "", "0.6", "0.6.0/../../evil", "https://example.com"])(
    "Should refuse release notes for %j",
    async version => {
      const { handle, openExternal } = control(version);
      await expect(handle("open_release_notes", {})).rejects.toThrow("no release notes");
      expect(openExternal).not.toHaveBeenCalled();
    }
  );

  it("Should open the config file and report why it could not open", async () => {
    await expect(control("0.6.0").handle("open_config", {})).resolves.toEqual({ opened: true });
    await expect(
      control("0.6.0", "No application opens this file.").handle("open_config", {})
    ).rejects.toThrow("No application opens this file.");
  });

  it("Should forward retry and refuse unknown actions", async () => {
    const { handle, forward } = control("0.6.0");
    await expect(handle("retry", {})).resolves.toEqual({ accepted: true });
    expect(forward).toHaveBeenCalledWith("retry", {});
    await expect(handle("format_disk", {})).rejects.toThrow("not supported");
  });

  it("Should accept prerelease and v-prefixed versions", () => {
    expect(releaseNotesURL("v0.7.0-beta.2")).toBe(
      "https://github.com/compozy/compozy/releases/tag/v0.7.0-beta.2"
    );
  });
});
