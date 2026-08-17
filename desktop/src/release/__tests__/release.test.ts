import { createServer } from "node:http";
import type { AddressInfo } from "node:net";

import { describe, expect, it } from "vitest";

import packageMetadata from "../../../package.json";
import { shouldDisableDifferentialDownload } from "../cross-arch";
import { assertMacZipSymlinks, parseZipInfoLongListing, requiredMacZipSymlinks } from "../mac-zip";
import { appleIDNotaryArguments } from "../notary-auth";
import {
  mergeMacUpdateManifests,
  parseUpdateManifest,
  rewriteUpdateManifestURLs,
} from "../mac-manifest";
import { buildReleaseConfig, validateReleaseConfig } from "../release-config";
import { parseBooleanFlag, parseReleaseScriptFlags } from "../release-script-flags";
import { resolveDesktopBuildChannel } from "../build-channel";

// Invariant: release policy emits only the signed, architecture-complete beta inventory.
// Owner: desktop release policy. Canonical suite: release.test.ts.
describe("desktop release policy", () => {
  it("Should bind packaged builds to the explicit release channel", () => {
    expect(resolveDesktopBuildChannel("beta")).toBe("beta");
    expect(resolveDesktopBuildChannel(undefined)).toBe("development");
    expect(() => resolveDesktopBuildChannel("stable")).toThrow("COMPOZY_RELEASE_CHANNEL");
  });

  it("Should produce per-architecture builder targets and the raw beta channel", () => {
    expect(
      buildReleaseConfig({
        arch: "arm64",
        channel: "beta",
        notarize: true,
        platform: "mac",
        version: "1.2.0-beta.1",
      })
    ).toMatchObject({
      dmg: { sign: true },
      extraMetadata: { version: "1.2.0-beta.1" },
      mac: { detectUpdateChannel: false, notarize: true, target: ["dmg", "zip"] },
      publish: {
        provider: "generic",
        url: "https://raw.githubusercontent.com/compozy/compozy/channel-beta/desktop/",
      },
    });
  });

  it("Should produce Linux targets and reject invalid release matrices", () => {
    expect(packageMetadata).toMatchObject({
      desktopName: "compozyos.desktop",
      homepage: "https://compozy.com",
    });
    expect(
      buildReleaseConfig({
        arch: "x64",
        channel: "beta",
        notarize: false,
        platform: "linux",
        version: "1.2.0-beta.1+build.7",
      })
    ).toMatchObject({
      linux: {
        artifactName: "CompozyOS-${version}-linux-x64.${ext}",
        detectUpdateChannel: false,
        executableName: "compozyos",
        maintainer: "Compozy",
        syncDesktopName: true,
        target: ["AppImage", "deb"],
      },
    });
    for (const input of [
      { arch: "x64", platform: "mac", notarize: false, version: "v1.2.0" },
      { arch: "x64", platform: "windows", notarize: false, version: "1.2.0" },
      { arch: "ia32", platform: "mac", notarize: false, version: "1.2.0" },
      { arch: "arm64", platform: "linux", notarize: false, version: "1.2.0" },
      { arch: "x64", platform: "linux", notarize: true, version: "1.2.0" },
    ]) {
      expect(() => validateReleaseConfig({ channel: "beta", ...input } as never)).toThrowError();
    }
  });

  it("Should merge mac manifests without losing architecture routing", () => {
    const manifest = (arch: string) => `version: 1.2.0-beta.1
files:
  - url: CompozyOS-1.2.0-beta.1-mac-${arch}.zip
    sha512: ${arch}
    size: 10
releaseDate: 2026-08-16T12:00:00Z
    `;
    const merged = mergeMacUpdateManifests(manifest("arm64"), manifest("x64"));
    expect(parseUpdateManifest(merged)).toEqual({
      version: "1.2.0-beta.1",
      releaseDate: "2026-08-16T12:00:00Z",
      files: [
        {
          url: "CompozyOS-1.2.0-beta.1-mac-arm64.zip",
          sha512: "arm64",
          size: 10,
        },
        { url: "CompozyOS-1.2.0-beta.1-mac-x64.zip", sha512: "x64", size: 10 },
      ],
    });
    const published = rewriteUpdateManifestURLs(
      merged,
      "https://github.com/compozy/compozy/releases/download/v1.2.0-beta.1"
    );
    expect(parseUpdateManifest(published).files).toEqual([
      {
        url: "https://github.com/compozy/compozy/releases/download/v1.2.0-beta.1/CompozyOS-1.2.0-beta.1-mac-arm64.zip",
        sha512: "arm64",
        size: 10,
      },
      {
        url: "https://github.com/compozy/compozy/releases/download/v1.2.0-beta.1/CompozyOS-1.2.0-beta.1-mac-x64.zip",
        sha512: "x64",
        size: 10,
      },
    ]);
  });

  it("Should reject invalid or conflicting update manifests", () => {
    expect(() => rewriteUpdateManifestURLs("null", "https://example.com")).toThrow(
      "must be an object"
    );
    expect(() => rewriteUpdateManifestURLs("version: 1.0.0", "https://example.com")).toThrow(
      "missing version"
    );
    expect(() =>
      rewriteUpdateManifestURLs(
        "version: 1.0.0\nreleaseDate: now\nfiles:\n  - invalid\n",
        "https://example.com"
      )
    ).toThrow("file is invalid");
    expect(() =>
      rewriteUpdateManifestURLs(
        "version: 1.0.0\nreleaseDate: now\nfiles:\n  - url: app.zip\n",
        "https://example.com"
      )
    ).toThrow("file is incomplete");
    expect(() =>
      rewriteUpdateManifestURLs("version: 1.0.0\nreleaseDate: now\nfiles: []\n", "relative")
    ).toThrow("absolute URL");

    const manifest = (version: string, sha512: string, date: string) => `version: "${version}"
files:
  - url: app.zip
    sha512: ${sha512}
    size: 10
releaseDate: "${date}"
`;
    expect(() => mergeMacUpdateManifests(manifest("1", "a", "2"), manifest("2", "a", "1"))).toThrow(
      "same version"
    );
    expect(() => mergeMacUpdateManifests(manifest("1", "a", "2"), manifest("1", "b", "1"))).toThrow(
      "Conflicting"
    );
    expect(mergeMacUpdateManifests(manifest("1", "a", "1"), manifest("1", "a", "2"))).toContain(
      'releaseDate: "2"'
    );
  });

  it("Should refuse a mac update zip whose framework symlink was dereferenced", () => {
    const requiredSymlinks = requiredMacZipSymlinks("CompozyOS.app");
    const entries = new Map(requiredSymlinks.map(path => [path, "lrwxr-xr-x"]));
    const corruptedSymlink = requiredSymlinks[0];
    if (!corruptedSymlink) throw new Error("The macOS symlink policy is empty.");
    entries.set(corruptedSymlink, "-rwxr-xr-x");
    expect(() => assertMacZipSymlinks("CompozyOS.app", entries)).toThrow(/lost required symlink/u);
    expect(() => requiredMacZipSymlinks("CompozyOS")).toThrow("must contain one app bundle");
  });

  it("Should reject unknown duplicate and invalid boolean release flags", () => {
    expect(() => parseReleaseScriptFlags(["--unknown", "value"], ["version"], ["version"])).toThrow(
      "Unknown"
    );
    expect(() =>
      parseReleaseScriptFlags(
        ["--version", "1.0.0", "--version", "1.0.1"],
        ["version"],
        ["version"]
      )
    ).toThrow("Duplicate");
    expect(() => parseBooleanFlag("yes", "notarize")).toThrow("true or false");
  });

  it("Should fail closed while preparing Apple ID notarization", () => {
    const environment = {
      APPLE_APP_SPECIFIC_PASSWORD: "app-password",
      APPLE_ID: "release@example.com",
      APPLE_TEAM_ID: "ABCDE12345",
    };
    expect(appleIDNotaryArguments(environment)).toEqual([
      "--apple-id",
      "release@example.com",
      "--password",
      "app-password",
      "--team-id",
      "ABCDE12345",
    ]);
    for (const missing of Object.keys(environment)) {
      expect(() => appleIDNotaryArguments({ ...environment, [missing]: "" })).toThrow(missing);
    }
  });

  it("Should parse zipinfo metadata separately from paths", () => {
    const listing = [
      "drwxr-xr-x  3.0 unx        0 bx        0 stor 26-Aug-16 12:24 CompozyOS.app/",
      "lrwxr-xr-x  3.0 unx       21 bx       21 stor 26-Aug-16 12:25 CompozyOS.app/Contents/Frameworks/Electron Framework.framework/Electron Framework",
    ].join("\n");
    expect(parseZipInfoLongListing(listing)).toEqual(
      new Map([
        ["CompozyOS.app/", "drwxr-xr-x"],
        [
          "CompozyOS.app/Contents/Frameworks/Electron Framework.framework/Electron Framework",
          "lrwxr-xr-x",
        ],
      ])
    );
  });

  it("Should disable differential downloads only for an x64 app on Apple Silicon", () => {
    expect(shouldDisableDifferentialDownload("x64", "arm64")).toBe(true);
    expect(shouldDisableDifferentialDownload("arm64", "arm64")).toBe(false);
    expect(shouldDisableDifferentialDownload("x64", "x64")).toBe(false);
  });

  it("Should rehearse N to N+1 through the generic provider HTTP path", async () => {
    const manifest = (version: string) => `version: ${version}
files:
  - url: CompozyOS-${version}-mac-arm64.zip
    sha512: arm64-${version}
    size: 10
  - url: CompozyOS-${version}-mac-x64.zip
    sha512: x64-${version}
    size: 10
releaseDate: 2026-08-16T12:00:00Z
`;
    let liveManifest = manifest("1.0.0-beta.1");
    const server = createServer((request, response) => {
      if (request.url === "/channel-beta/desktop/latest-mac.yml") {
        response.writeHead(200, { "content-type": "text/yaml" }).end(liveManifest);
        return;
      }
      if (request.url === "/CompozyOS-1.0.0-beta.2-mac-x64.zip") {
        response.writeHead(200, { "content-type": "application/zip" }).end("candidate");
        return;
      }
      response.writeHead(404).end();
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    try {
      const address = server.address() as AddressInfo;
      const provider = `http://127.0.0.1:${address.port}/channel-beta/desktop`;
      expect(await (await fetch(`${provider}/latest-mac.yml`)).text()).toContain("1.0.0-beta.1");

      liveManifest = manifest("1.0.0-beta.2");
      const candidate = await (await fetch(`${provider}/latest-mac.yml`)).text();
      expect(candidate).toContain("CompozyOS-1.0.0-beta.2-mac-arm64.zip");
      expect(candidate).toContain("CompozyOS-1.0.0-beta.2-mac-x64.zip");
      const packageResponse = await fetch(
        `http://127.0.0.1:${address.port}/CompozyOS-1.0.0-beta.2-mac-x64.zip`
      );
      expect(packageResponse.status).toBe(200);
      expect(await packageResponse.text()).toBe("candidate");
    } finally {
      await new Promise<void>((resolve, reject) =>
        server.close(error => (error ? reject(error) : resolve()))
      );
    }
  });
});
