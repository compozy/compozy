import { beforeEach, describe, expect, it, vi } from "vitest";
import { loadChangelogReleases } from "../changelog/github-client";
import type { ChangelogRelease } from "../changelog/types";
import {
  LATEST_RELEASE_URL,
  detectVisitorPlatform,
  getDesktopRelease,
  primaryTargetFor,
  selectDesktopRelease,
} from "../release/desktop-downloads";

vi.mock("../changelog/github-client", () => ({
  loadChangelogReleases: vi.fn(),
}));

const DESKTOP_ASSETS = [
  "mac-arm64.dmg",
  "mac-x64.dmg",
  "linux-x64.AppImage",
  "linux-x64.deb",
] as const;

function assetUrl(version: string, name: string) {
  return `https://github.com/compozy/compozy/releases/download/${version}/${name}`;
}

function release(
  version: string,
  { prerelease = false, desktop = true }: { prerelease?: boolean; desktop?: boolean } = {}
): ChangelogRelease {
  const bare = version.replace(/^v/, "");
  const names = [
    "checksums.txt",
    "compozy_windows_x86_64.zip",
    ...(desktop ? DESKTOP_ASSETS.map(suffix => `CompozyOS-${bare}-${suffix}`) : []),
  ];
  return {
    version,
    name: null,
    body: "",
    summary: "",
    publishedAt: "2026-11-06T02:25:39Z",
    channel: prerelease ? "beta" : "stable",
    prerelease,
    url: `https://github.com/compozy/compozy/releases/tag/${version}`,
    sourceArchiveUrl: "https://example.invalid/tarball",
    sourceZipUrl: "https://example.invalid/zipball",
    assets: names.map(name => ({
      name,
      url: assetUrl(version, name),
      size: 1024,
      contentType: "application/octet-stream",
      downloadCount: 0,
    })),
    categories: [],
    headings: [],
    pullRequestNumbers: [],
  };
}

describe("desktop release selection", () => {
  beforeEach(() => {
    vi.mocked(loadChangelogReleases).mockReset();
  });

  it("links every desktop package of the newest stable release by its published asset URL", () => {
    const selected = selectDesktopRelease([
      release("v0.5.2"),
      release("v0.6.0-beta.1", { prerelease: true }),
      release("v0.5.3"),
    ]);

    expect(selected.status).toBe("ready");
    if (selected.status !== "ready") return;
    expect(selected.tag).toBe("v0.5.3");
    expect(selected.downloads.map(item => [item.target.id, item.url])).toEqual([
      ["mac-arm64", assetUrl("v0.5.3", "CompozyOS-0.5.3-mac-arm64.dmg")],
      ["mac-x64", assetUrl("v0.5.3", "CompozyOS-0.5.3-mac-x64.dmg")],
      ["linux-appimage", assetUrl("v0.5.3", "CompozyOS-0.5.3-linux-x64.AppImage")],
      ["linux-deb", assetUrl("v0.5.3", "CompozyOS-0.5.3-linux-x64.deb")],
    ]);
    expect(selected.checksums?.url).toBe(assetUrl("v0.5.3", "checksums.txt"));
    expect(selected.windowsCommand?.url).toBe(assetUrl("v0.5.3", "compozy_windows_x86_64.zip"));
  });

  it("skips a stable release that published no desktop packages", () => {
    const selected = selectDesktopRelease([
      release("v0.5.4", { desktop: false }),
      release("v0.5.3"),
    ]);

    expect(selected.status === "ready" && selected.tag).toBe("v0.5.3");
  });

  it("falls back to the latest-release page when GitHub is unavailable", async () => {
    vi.mocked(loadChangelogReleases).mockResolvedValue({
      status: "unavailable",
      error: { kind: "network", message: "GitHub could not be reached" },
    });

    await expect(getDesktopRelease()).resolves.toEqual({
      status: "fallback",
      releaseUrl: LATEST_RELEASE_URL,
    });
  });

  it("falls back when no stable release carries desktop packages", () => {
    expect(selectDesktopRelease([release("v0.6.0-beta.1", { prerelease: true })])).toEqual({
      status: "fallback",
      releaseUrl: LATEST_RELEASE_URL,
    });
  });
});

const MAC_UA =
  "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36";

describe("visitor platform detection", () => {
  it.each([
    [{ userAgent: MAC_UA, architecture: "arm" }, "mac-arm64"],
    [{ userAgent: MAC_UA, architecture: "x86" }, "mac-x64"],
    [
      { userAgent: MAC_UA, gpuRenderer: "ANGLE (Apple, ANGLE Metal Renderer: Apple M3 Pro)" },
      "mac-arm64",
    ],
    [{ userAgent: MAC_UA, gpuRenderer: "Intel(R) Iris(TM) Plus Graphics 655" }, "mac-x64"],
    [{ userAgent: MAC_UA, gpuRenderer: "Apple GPU" }, "mac"],
    [{ userAgent: MAC_UA, maxTouchPoints: 5 }, "mobile"],
    [{ userAgent: "Mozilla/5.0 (X11; Linux x86_64) Firefox/140.0" }, "linux"],
    [{ userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/142.0" }, "windows"],
    [{ userAgent: "Mozilla/5.0 (Linux; Android 15) Chrome/142.0 Mobile" }, "mobile"],
    [{ userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 19_0 like Mac OS X)" }, "mobile"],
  ] as const)("detects %o as %s", (signals, expected) => {
    expect(detectVisitorPlatform(signals)).toBe(expected);
  });

  it("offers the matching package first and no desktop package where none exists", () => {
    expect(primaryTargetFor("mac-arm64")).toBe("mac-arm64");
    expect(primaryTargetFor("mac")).toBe("mac-arm64");
    expect(primaryTargetFor("mac-x64")).toBe("mac-x64");
    expect(primaryTargetFor("linux")).toBe("linux-appimage");
    expect(primaryTargetFor("windows")).toBeNull();
    expect(primaryTargetFor("mobile")).toBeNull();
  });
});
