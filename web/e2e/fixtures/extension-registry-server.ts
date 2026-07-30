import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer, type Server } from "node:http";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const DEFAULT_HOST = "127.0.0.1";

/** One published extension release the daemon's GitHub source can resolve and download. */
export interface BrowserExtensionReleaseSeed {
  /** Release tag, e.g. `v0.1.0`. */
  tag: string;
  /** Manifest version installed from this release, e.g. `0.1.0`. */
  version: string;
}

export interface BrowserExtensionRegistrySeed {
  repository: string;
  extensionName: string;
  description?: string;
  releases: BrowserExtensionReleaseSeed[];
}

interface RegistryAsset {
  archive: Buffer;
  digest: string;
  id: number;
  name: string;
}

interface RegistryRelease {
  assets: RegistryAsset[];
  id: number;
  tag: string;
}

export interface ExtensionRegistryTestServer {
  baseURL: string;
  server: Server;
  digestFor(tag: string): string;
}

/**
 * Read-only mirror of the GitHub release endpoints the daemon's extension source uses:
 * latest / by-tag / list plus asset and digest-sidecar downloads. Publishing is out of scope —
 * releases are seeded in-process so the browser lane never reaches the network.
 */
export async function startExtensionRegistryServer(
  seed: BrowserExtensionRegistrySeed | undefined
): Promise<ExtensionRegistryTestServer | undefined> {
  if (seed === undefined) return undefined;
  const [owner, repo] = seed.repository.split("/");
  if (!owner || !repo) {
    throw new Error(`extension registry repository ${seed.repository} must use owner/repo format`);
  }
  const releases: RegistryRelease[] = [];
  let nextId = 0;
  for (const release of seed.releases) {
    const archive = await buildExtensionArchive(seed, release);
    nextId += 1;
    releases.unshift({
      assets: [
        {
          archive,
          digest: createHash("sha256").update(archive).digest("hex"),
          id: nextId,
          name: `${seed.extensionName}_${release.tag}.tar.gz`,
        },
      ],
      id: nextId,
      tag: release.tag,
    });
  }

  const server = createServer((request, response) => {
    const requestURL = new URL(request.url ?? "/", `http://${DEFAULT_HOST}`);
    const baseURL = `http://${DEFAULT_HOST}:${port}`;
    handleRegistryRequest({
      baseURL,
      pathname: requestURL.pathname,
      releases,
      repository: `${owner}/${repo}`,
      respond: (status, body, contentType) => {
        response.statusCode = status;
        response.setHeader("content-type", contentType);
        response.end(body);
      },
    });
  });

  await new Promise<void>((resolve, reject) => {
    const cleanup = () => {
      server.off("error", handleError);
      server.off("listening", handleListening);
    };
    const handleError = (error: Error) => {
      cleanup();
      reject(error);
    };
    const handleListening = () => {
      cleanup();
      resolve();
    };
    server.once("error", handleError);
    server.once("listening", handleListening);
    server.listen(0, DEFAULT_HOST);
  });

  const address = server.address();
  if (address === null || typeof address === "string") {
    await closeExtensionRegistryServer(server);
    throw new Error("failed to resolve browser extension registry server address");
  }
  const port = address.port;
  return {
    baseURL: `http://${DEFAULT_HOST}:${port}`,
    digestFor: tag => {
      const release = releases.find(candidate => candidate.tag === tag);
      if (!release?.assets[0]) throw new Error(`unknown extension release tag ${tag}`);
      return release.assets[0].digest;
    },
    server,
  };
}

function handleRegistryRequest(input: {
  baseURL: string;
  pathname: string;
  releases: RegistryRelease[];
  repository: string;
  respond: (status: number, body: string | Buffer, contentType: string) => void;
}): void {
  const releasesPath = `/repos/${input.repository}/releases`;
  const json = (status: number, payload: unknown) =>
    input.respond(status, JSON.stringify(payload), "application/json");

  if (input.pathname === `${releasesPath}/latest`) {
    const release = input.releases[0];
    if (!release) {
      json(404, { message: "release not found" });
      return;
    }
    json(200, releasePayload(release, input.baseURL));
    return;
  }
  if (input.pathname.startsWith(`${releasesPath}/tags/`)) {
    const tag = decodeURIComponent(input.pathname.slice(`${releasesPath}/tags/`.length));
    const release = input.releases.find(candidate => candidate.tag === tag);
    if (!release) {
      json(404, { message: "release not found" });
      return;
    }
    json(200, releasePayload(release, input.baseURL));
    return;
  }
  if (input.pathname === releasesPath) {
    json(
      200,
      input.releases.map(release => releasePayload(release, input.baseURL))
    );
    return;
  }
  if (input.pathname.startsWith("/assets/")) {
    const [rawId, sidecar] = input.pathname.slice("/assets/".length).split("/");
    const asset = input.releases
      .flatMap(release => release.assets)
      .find(candidate => String(candidate.id) === rawId);
    if (!asset) {
      json(404, { message: "asset not found" });
      return;
    }
    if (sidecar === "sha256") {
      input.respond(200, `${asset.digest}  ${asset.name}\n`, "application/octet-stream");
      return;
    }
    input.respond(200, asset.archive, "application/gzip");
    return;
  }
  json(404, { message: "not found" });
}

function releasePayload(release: RegistryRelease, baseURL: string) {
  return {
    assets: release.assets.flatMap(asset => [
      {
        browser_download_url: `${baseURL}/assets/${asset.id}`,
        content_type: "application/gzip",
        download_count: 1,
        id: asset.id,
        name: asset.name,
        size: asset.archive.byteLength,
        url: `${baseURL}/assets/${asset.id}`,
      },
      {
        browser_download_url: `${baseURL}/assets/${asset.id}/sha256`,
        content_type: "application/octet-stream",
        download_count: 0,
        id: asset.id * 1000,
        name: `${asset.name}.sha256`,
        size: asset.digest.length,
        url: `${baseURL}/assets/${asset.id}/sha256`,
      },
    ]),
    author: { login: "acme" },
    draft: false,
    html_url: `${baseURL}/releases/${release.tag}`,
    id: release.id,
    name: release.tag,
    prerelease: false,
    tag_name: release.tag,
    upload_url: `${baseURL}/uploads/${release.id}/assets{?name,label}`,
  };
}

async function buildExtensionArchive(
  seed: BrowserExtensionRegistrySeed,
  release: BrowserExtensionReleaseSeed
): Promise<Buffer> {
  const stageRoot = await mkdtemp(path.join(os.tmpdir(), "compozy-browser-extension-release-"));
  const packageDir = path.join(stageRoot, seed.extensionName);
  await mkdir(packageDir, { recursive: true });
  await writeFile(
    path.join(packageDir, "extension.toml"),
    [
      "[extension]",
      `name = ${JSON.stringify(seed.extensionName)}`,
      `version = ${JSON.stringify(release.version)}`,
      `description = ${JSON.stringify(seed.description ?? "Browser lane extension release")}`,
      'min_compozy_version = "0.0.0"',
      "",
    ].join("\n"),
    "utf8"
  );
  const archivePath = path.join(stageRoot, "package.tar.gz");
  await execFileAsync("tar", ["-czf", archivePath, "-C", packageDir, "extension.toml"]);
  return await readFile(archivePath);
}

export async function closeExtensionRegistryServer(server: Server | undefined): Promise<void> {
  if (server === undefined || !server.listening) return;
  await new Promise<void>((resolve, reject) => {
    server.close(error => {
      if (error) {
        reject(error);
        return;
      }
      resolve();
    });
  });
}
