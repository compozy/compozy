import { createServer } from "node:http";
import type { AddressInfo } from "node:net";

import { describe, expect, it } from "vitest";

import { shouldDisableDifferentialDownload } from "../cross-arch";
import { assertMacZipSymlinks, requiredMacZipSymlinks } from "../mac-zip";
import { mergeMacUpdateManifests, rewriteUpdateManifestURLs } from "../mac-manifest";
import { buildReleaseConfig } from "../release-config";

describe("desktop release policy", () => {
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
      extraMetadata: { version: "1.2.0-beta.1" },
      mac: { notarize: true, target: ["dmg", "zip"] },
      publish: {
        provider: "generic",
        url: "https://raw.githubusercontent.com/compozy/compozy/channel-beta/desktop/",
      },
    });
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
    expect(merged).toContain("mac-arm64.zip");
    expect(merged).toContain("mac-x64.zip");
    const published = rewriteUpdateManifestURLs(
      merged,
      "https://github.com/compozy/compozy/releases/download/v1.2.0-beta.1"
    );
    expect(published).toContain(
      "https://github.com/compozy/compozy/releases/download/v1.2.0-beta.1/CompozyOS-1.2.0-beta.1-mac-arm64.zip"
    );
  });

  it("Should refuse a mac update zip whose framework symlink was dereferenced", () => {
    const entries = new Map(
      requiredMacZipSymlinks("CompozyOS.app").map(path => [path, "lrwxr-xr-x"])
    );
    entries.set(requiredMacZipSymlinks("CompozyOS.app")[0]!, "-rwxr-xr-x");
    expect(() => assertMacZipSymlinks("CompozyOS.app", entries)).toThrow(/lost required symlink/u);
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
