import { execFile } from "node:child_process";
import { createHash } from "node:crypto";
import { createServer, type Server } from "node:http";
import { mkdir, mkdtemp, readFile, writeFile } from "node:fs/promises";
import os from "node:os";
import path from "node:path";
import { promisify } from "node:util";

const execFileAsync = promisify(execFile);
const DEFAULT_HOST = "127.0.0.1";

/**
 * Package layout an archived release carries. `native` is the Compozy manifest; `agent-plugin` is
 * the portable standard layout; `client-layout` is the drifted Claude Code shape the installer
 * refuses by design.
 */
export type BrowserExtensionLayout = "native" | "agent-plugin" | "client-layout";

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
  /** Defaults to the native Compozy manifest layout. */
  layout?: BrowserExtensionLayout;
  releases: BrowserExtensionReleaseSeed[];
}

const PLUGIN_SCHEMA_ID = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json";
const MCP_SCHEMA_ID = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json";
/** The one server the conformance ladder records as skipped: `sse` is out of scope by decision. */
export const SKIPPED_PLUGIN_SERVER_NAME = "legacy-events";
export const INGESTED_PLUGIN_SKILL_NAME = "deploy-check";

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
  const entries = await writePackageLayout(seed, release, packageDir);
  const archivePath = path.join(stageRoot, "package.tar.gz");
  await execFileAsync("tar", ["-czf", archivePath, "-C", packageDir, ...entries]);
  return await readFile(archivePath);
}

/** Writes the seeded layout and returns the archive members, so each layout stays self-describing. */
async function writePackageLayout(
  seed: BrowserExtensionRegistrySeed,
  release: BrowserExtensionReleaseSeed,
  packageDir: string
): Promise<string[]> {
  const description = seed.description ?? "Browser lane extension release";
  switch (seed.layout ?? "native") {
    case "agent-plugin":
      return await writeAgentPluginLayout(seed, release, packageDir, description);
    case "client-layout":
      return await writeClientPluginLayout(seed, release, packageDir, description);
    case "native":
      await writeFile(
        path.join(packageDir, "extension.toml"),
        [
          "[extension]",
          `name = ${JSON.stringify(seed.extensionName)}`,
          `version = ${JSON.stringify(release.version)}`,
          `description = ${JSON.stringify(description)}`,
          'min_compozy_version = "0.0.0"',
          "",
        ].join("\n"),
        "utf8"
      );
      return ["extension.toml"];
  }
}

/**
 * Standard layout: root `plugin.json`, one discoverable skill, and an `mcp.json` declaring one
 * synthesizable stdio server plus one `sse` server the ladder records as a skip. The skip is the
 * point — it is what the inventory panel's Skipped section has to render.
 */
async function writeAgentPluginLayout(
  seed: BrowserExtensionRegistrySeed,
  release: BrowserExtensionReleaseSeed,
  packageDir: string,
  description: string
): Promise<string[]> {
  await writeFile(
    path.join(packageDir, "plugin.json"),
    `${JSON.stringify(
      {
        $schema: PLUGIN_SCHEMA_ID,
        description,
        name: seed.extensionName,
        version: release.version,
      },
      null,
      2
    )}\n`,
    "utf8"
  );
  await writeFile(
    path.join(packageDir, "mcp.json"),
    `${JSON.stringify(
      {
        $schema: MCP_SCHEMA_ID,
        mcpServers: {
          "tools-api": { type: "streamable-http", url: "https://tools.example.test/mcp" },
          [SKIPPED_PLUGIN_SERVER_NAME]: { type: "sse", url: "https://events.example.test/sse" },
        },
      },
      null,
      2
    )}\n`,
    "utf8"
  );
  const skillDir = path.join(packageDir, "skills", INGESTED_PLUGIN_SKILL_NAME);
  await mkdir(skillDir, { recursive: true });
  await writeFile(
    path.join(skillDir, "SKILL.md"),
    [
      "---",
      `name: ${INGESTED_PLUGIN_SKILL_NAME}`,
      "description: Run the deployment preflight checks this package ships.",
      "---",
      "",
      "# Deploy check",
      "",
      "Run the preflight checks before promoting a release.",
      "",
    ].join("\n"),
    "utf8"
  );
  return ["plugin.json", "mcp.json", "skills"];
}

/** Claude Code's client-specific shape: a `.claude-plugin/plugin.json` with no conformant root. */
async function writeClientPluginLayout(
  seed: BrowserExtensionRegistrySeed,
  release: BrowserExtensionReleaseSeed,
  packageDir: string,
  description: string
): Promise<string[]> {
  const clientDir = path.join(packageDir, ".claude-plugin");
  await mkdir(clientDir, { recursive: true });
  await writeFile(
    path.join(clientDir, "plugin.json"),
    `${JSON.stringify(
      { description, name: seed.extensionName, version: release.version },
      null,
      2
    )}\n`,
    "utf8"
  );
  return [".claude-plugin"];
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
