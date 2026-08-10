import { createHash } from "node:crypto";
import {
  chmodSync,
  existsSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import { basename, join, relative, resolve } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { describe, expect, it, vi } from "vitest";
import { GET as getInstallScriptRoute } from "../../app/install.sh/route";
import { siteRoot } from "../content-test-utils";
import { INSTALL_SCRIPT_TEMPLATE } from "../install/template-content";
import { INSTALL_VERSION_PLACEHOLDER, renderInstallScript } from "../install/render-install-script";
import {
  NPM_INSTALL_COMMAND,
  VERIFIED_INSTALLER_COMMAND,
  curlEnvInstallCommand,
  curlPinnedInstallCommand,
  goInstallCommand,
} from "../release/install-commands";

vi.mock("../release/current-release", () => ({
  getCurrentRelease: vi.fn(async () => ({ tag: "v9.9.9-beta.9", source: "github" as const })),
}));

const repoRoot = resolve(siteRoot, "..", "..");
const installTemplatePath = resolve(repoRoot, "scripts/install.template.sh");
const renderScriptPath = resolve(repoRoot, "scripts/render-install-script.sh");
const installPagePath = resolve(siteRoot, "content/docs/getting-started/installation.mdx");
const migrationPagePath = resolve(siteRoot, "content/docs/migration/index.mdx");
const launchPostPath = resolve(siteRoot, "content/blog/posts/introducing-compozyos.mdx");
const landingInstallPath = resolve(siteRoot, "components/landing/install-section.tsx");
const readmePath = resolve(repoRoot, "README.md");
const rootMigrationGuidePath = resolve(repoRoot, "MIGRATION_GUIDE.md");
const homebrewNoticePath = resolve(repoRoot, "docs/release/homebrew-beta-notice.md");
const releaseHeaderPath = resolve(repoRoot, ".goreleaser.release-header.md.tmpl");
const releaseFooterPath = resolve(repoRoot, ".goreleaser.release-footer.md.tmpl");
const cliffPath = resolve(repoRoot, "cliff.toml");

const resolvedRouteTag = "v9.9.9-beta.9";
const renderedTestTag = "v9.9.9";
const sourceInstallCommand = "go build -o ./bin/compozy .";
const firstSessionCommand = "compozy session new --agent general --name first-run";
const historicalLaunchBeta = "v0.3.0-beta.1";
const latestReleaseUrl = "https://github.com/compozy/compozy/releases/latest";
const installOptions = ["--version", "--dir", "--skip-bootstrap", "--dry-run", "--help"];
const installEnvVars = ["COMPOZY_VERSION", "COMPOZY_INSTALL_DIR", "COMPOZY_SKIP_BOOTSTRAP"];
const cosignVersion = "v3.1.3";
const cosignDigests = {
  "darwin/x64": "2347488e5d5b25336644024dfeca5601b190e91197a71a917bda44744aff106c",
  "darwin/arm64": "5cf948c2f4dfe59687bdd0b8523709067383e03982cc543475c8a7dc70e92a76",
  "linux/x64": "4629c757b7618056f8ddd7e2625ae9fdd94c0372a65049520bc7d9df9efc7f71",
  "linux/arm64": "c5d324e091826b0d7a78eb16fef316450b4eb9aaec045611c08ba06f5e73220a",
} as const;
const installerReleaseGuaranteeSnippets = [
  VERIFIED_INSTALLER_COMMAND,
  "Requires:",
  "curl, tar, and sha256sum or shasum.",
  "Uses compatible local cosign v3 when available; otherwise downloads a pinned temporary cosign verifier.",
  'COSIGN_VERSION="v3.1.3"',
  'COSIGN_BASE_URL="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}"',
  "cosign-darwin-amd64",
  "cosign-darwin-arm64",
  "cosign-linux-amd64",
  "cosign-linux-arm64",
  ...Object.values(cosignDigests),
  "resolve_cosign()",
  'candidate_cosign="$(command -v cosign)"',
  'candidate_version="$("$candidate_cosign" version 2>/dev/null)"',
  '*"GitVersion:"*"v3."*)',
  'COSIGN_BIN="$candidate_cosign"',
  'curl -fsSL "$COSIGN_URL" -o "$COSIGN_PATH"',
  'verify_file_sha256 "$COSIGN_PATH" "$COSIGN_SHA256" "cosign verifier"',
  'COSIGN_BIN="$COSIGN_PATH"',
  'BUNDLE_URL="${BASE_URL}/checksums.txt.sigstore.json"',
  'log "verifying checksum provenance"',
  '"$COSIGN_BIN" verify-blob "$CHECKSUM_PATH"',
  '--bundle "$BUNDLE_PATH"',
  '--certificate-identity-regexp "$COSIGN_CERT_IDENTITY_REGEXP"',
  '--certificate-oidc-issuer "$COSIGN_CERT_OIDC_ISSUER"',
  'CHECKSUM_CMD="sha256sum"',
  'CHECKSUM_CMD="shasum"',
  "shasum -a 256 -c - >/dev/null",
];
const installerCriticalErrorSnippets = [
  "release version must be an explicit v-prefixed tag",
  "unsupported operating system:",
  "unsupported architecture:",
  "unsupported cosign verifier platform:",
  "curl is required",
  "tar is required",
  "checksum mismatch",
  "sha256sum or shasum is required to verify the download",
  "checksums.txt does not include",
  "archive did not contain a compozy binary",
  "warning: ${INSTALL_DIR} is not on PATH",
  "add it to PATH or run ${TARGET} directly",
  "no interactive terminal detected; run this next:",
  "compozy install",
];
const ttyPermissionProbePattern =
  /\[\s*-[rwe]\s+["']?\/dev\/tty["']?\s*\]|\btest\s+-[rwe]\s+["']?\/dev\/tty["']?/;
const ttyOpenProbePattern =
  /^\s*(?:if|elif)\s+!?\s*[^\n]*<\s*\/dev\/tty[^\n]*>\s*\/dev\/tty[^\n]*;\s*then/m;
const credentialEnvPattern =
  /(?:API_?KEY|ACCESS_KEY|PRIVATE_KEY|TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|AUTH|COOKIE|SESSION)|_KEY$/i;
const pinnedBetaLiteralPattern = /-beta\.\d/;
type InstallEnv = Record<string, string | undefined>;

function readSiteFile(path: string): string {
  return readFileSync(path, "utf8");
}

function readInstallTemplate(): string {
  return readSiteFile(installTemplatePath);
}

function writeRenderedInstallScript(root: string, tag: string): string {
  const scriptPath = join(root, "install.sh");
  writeFileSync(scriptPath, renderInstallScript(readInstallTemplate(), tag), "utf8");
  chmodSync(scriptPath, 0o755);
  return scriptPath;
}

function runInstallScript(args: string[]) {
  const root = mkdtempSync(resolve(tmpdir(), "compozy-install-contract-"));
  try {
    const scriptPath = writeRenderedInstallScript(root, renderedTestTag);
    const installDir = join(root, "bin");
    return spawnSync("sh", [scriptPath, ...args, "--dir", installDir], {
      cwd: siteRoot,
      encoding: "utf8",
      env: hermeticInstallEnv(),
    });
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

function currentInstallPlatform() {
  if (process.platform !== "darwin" && process.platform !== "linux") {
    throw new Error("unsupported test platform: " + process.platform);
  }
  if (process.arch !== "x64" && process.arch !== "arm64") {
    throw new Error("unsupported test architecture: " + process.arch);
  }

  const os = process.platform;
  const archiveArch = process.arch === "x64" ? "x86_64" : "arm64";
  const cosignArch = process.arch === "x64" ? "amd64" : "arm64";
  const digestKey = (os + "/" + process.arch) as keyof typeof cosignDigests;

  return {
    archiveName: "compozy_" + os + "_" + archiveArch + ".tar.gz",
    cosignName: "cosign-" + os + "-" + cosignArch,
    cosignDigest: cosignDigests[digestKey],
  };
}

function sha256(data: Buffer | string): string {
  return createHash("sha256").update(data).digest("hex");
}

function createFixtureArchive(root: string): Buffer {
  const sourceDir = join(root, "archive-src");
  mkdirSync(sourceDir, { recursive: true });
  const binaryPath = join(sourceDir, "compozy");
  writeFileSync(
    binaryPath,
    '#!/bin/sh\nif [ "${1:-}" = "version" ]; then exit 0; fi\nexit 0\n',
    "utf8"
  );
  chmodSync(binaryPath, 0o755);

  const archivePath = join(root, "compozy.tar.gz");
  const result = spawnSync("tar", ["-czf", archivePath, "-C", sourceDir, "compozy"], {
    encoding: "utf8",
  });
  if (result.status !== 0) {
    throw new Error("tar failed: " + (result.stderr || result.stdout));
  }
  return readFileSync(archivePath);
}

function runInstallScriptAsync(
  scriptPath: string,
  args: string[],
  env: NodeJS.ProcessEnv
): Promise<{ status: number | null; stdout: string; stderr: string }> {
  return new Promise(resolveRun => {
    const child = spawn("sh", [scriptPath, ...args], {
      cwd: siteRoot,
      env,
      stdio: ["ignore", "pipe", "pipe"],
    });
    const stdout: Buffer[] = [];
    const stderr: Buffer[] = [];
    let settled = false;

    function finish(status: number | null) {
      if (settled) {
        return;
      }
      settled = true;
      resolveRun({
        status,
        stdout: Buffer.concat(stdout).toString("utf8"),
        stderr: Buffer.concat(stderr).toString("utf8"),
      });
    }

    child.stdout.on("data", chunk => stdout.push(Buffer.from(chunk)));
    child.stderr.on("data", chunk => stderr.push(Buffer.from(chunk)));
    child.on("error", error => {
      stderr.push(Buffer.from(error.message));
      finish(1);
    });
    child.on("close", status => finish(status));
  });
}

async function withFixtureServer<T>(
  routes: Map<string, Buffer | string>,
  run: (baseURL: string) => Promise<T>
): Promise<T> {
  const server = createServer((request, response) => {
    const routeName = basename((request.url ?? "").split("?")[0] ?? "");
    const body = routes.get(routeName);
    if (body === undefined) {
      response.statusCode = 404;
      response.end("missing fixture: " + routeName + "\n");
      return;
    }

    response.statusCode = 200;
    response.end(body);
  });

  await new Promise<void>((resolveListen, rejectListen) => {
    server.once("error", rejectListen);
    server.listen(0, "127.0.0.1", () => resolveListen());
  });

  try {
    const address = server.address() as AddressInfo;
    return await run("http://127.0.0.1:" + address.port);
  } finally {
    await new Promise<void>((resolveClose, rejectClose) => {
      server.close(error => {
        if (error) {
          rejectClose(error);
          return;
        }
        resolveClose();
      });
    });
  }
}

async function runFixtureInstallWithLocalCosign(root: string, localCosignBody: string) {
  const platform = currentInstallPlatform();
  const installDir = join(root, "bin");
  const toolDir = join(root, "tools");
  const downloadedArgsPath = join(root, "downloaded-cosign-args.txt");
  mkdirSync(toolDir, { recursive: true });

  const localCosignPath = join(toolDir, "cosign");
  writeFileSync(localCosignPath, localCosignBody, "utf8");
  chmodSync(localCosignPath, 0o755);

  const archiveBody = createFixtureArchive(root);
  const downloadedCosignBody =
    '#!/bin/sh\nprintf \'%s\\n\' "$@" > "' + downloadedArgsPath + '"\nexit 0\n';
  const routes = new Map<string, Buffer | string>([
    [platform.archiveName, archiveBody],
    ["checksums.txt", sha256(archiveBody) + "  " + platform.archiveName + "\n"],
    [
      "checksums.txt.sigstore.json",
      '{"mediaType":"application/vnd.dev.sigstore.bundle.v0.3+json"}\n',
    ],
    [platform.cosignName, downloadedCosignBody],
  ]);

  const result = await withFixtureServer(routes, async baseURL => {
    const script = renderInstallScript(readInstallTemplate(), renderedTestTag)
      .replace(
        'COSIGN_BASE_URL="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}"',
        'COSIGN_BASE_URL="' + baseURL + "/cosign/" + cosignVersion + '"'
      )
      .replace(platform.cosignDigest, sha256(downloadedCosignBody))
      .replace(
        'BASE_URL="https://github.com/${RELEASE_REPO}/releases/download/${VERSION}"',
        'BASE_URL="' + baseURL + '/releases/download/${VERSION}"'
      );
    const scriptPath = join(root, "install.sh");
    writeFileSync(scriptPath, script, "utf8");
    chmodSync(scriptPath, 0o755);

    return runInstallScriptAsync(
      scriptPath,
      ["--version", "v9.9.9", "--skip-bootstrap", "--dir", installDir],
      {
        HOME: root,
        PATH: toolDir + ":/usr/bin:/bin",
        TZ: "UTC",
        LANG: "C.UTF-8",
        LC_ALL: "C.UTF-8",
        LC_CTYPE: "C.UTF-8",
        NODE_ENV: "test",
      }
    );
  });

  return { downloadedArgsPath, result };
}

function hermeticInstallEnv(source: InstallEnv = process.env): NodeJS.ProcessEnv {
  const env: InstallEnv = {};
  for (const [key, value] of Object.entries(source)) {
    if (value === undefined || blocksHermeticInstallEnv(key)) {
      continue;
    }
    env[key] = value;
  }
  env.TZ = "UTC";
  env.LANG = "C.UTF-8";
  env.LC_ALL = "C.UTF-8";
  env.LC_CTYPE = "C.UTF-8";
  env.NODE_ENV ??= "test";
  env.COMPOZY_SKIP_BOOTSTRAP = "";
  return env as NodeJS.ProcessEnv;
}

function blocksHermeticInstallEnv(key: string): boolean {
  const normalized = key.trim().toUpperCase();
  return (
    normalized.startsWith("COMPOZY_") ||
    credentialEnvPattern.test(normalized) ||
    [
      "CLAUDE_CONFIG_DIR",
      "CODEX_HOME",
      "OPENCODE_CONFIG_DIR",
      "PI_CODING_AGENT_DIR",
      "PROVIDER_CODEX_HOME",
      "PROVIDER_HOME",
    ].includes(normalized)
  );
}

describe("public install contract", () => {
  it("keeps the installer template safe for the public curl entrypoint", () => {
    const script = readInstallTemplate();
    const downloadIndex = script.indexOf('curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_PATH"');
    const checksumDownloadIndex = script.indexOf('curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_PATH"');
    const bundleDownloadIndex = script.indexOf('curl -fsSL "$BUNDLE_URL" -o "$BUNDLE_PATH"');
    const provenanceVerifyIndex = script.indexOf("verifying checksum provenance");
    const checksumVerifyIndex = script.indexOf('log "verifying checksum"');
    const extractIndex = script.indexOf('tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR"');

    expect(script.startsWith("#!/bin/sh\nset -eu\n")).toBe(true);
    expect(script).toContain('RELEASE_REPO="compozy/compozy"');
    expect(script).not.toContain("COMPOZY_RELEASE_REPO");
    expect(script).toContain('BASE_URL="https://github.com/${RELEASE_REPO}/releases');
    expect(script).not.toContain("http://");
    expect(script).toContain('command -v curl >/dev/null 2>&1 || fail "curl is required"');
    expect(script).toContain('command -v tar >/dev/null 2>&1 || fail "tar is required"');
    expect(script).not.toContain('fail "cosign is required to verify release provenance"');
    expect(script).toContain('COSIGN_VERSION="v3.1.3"');
    expect(script).toContain("resolve_cosign()");
    expect(script).toContain("if command -v cosign >/dev/null 2>&1; then");
    expect(script).toContain('BUNDLE_URL="${BASE_URL}/checksums.txt.sigstore.json"');
    expect(script).toContain(
      "COSIGN_CERT_IDENTITY_REGEXP='^https://github\\.com/compozy/compozy/\\.github/workflows/release\\.yml@refs/heads/main$'"
    );
    expect(script).toContain(`VERSION="\${COMPOZY_VERSION:-${INSTALL_VERSION_PLACEHOLDER}}"`);
    expect(script).toContain("v[0-9][A-Za-z0-9._-]*)");
    expect(script).toContain(
      'COSIGN_CERT_OIDC_ISSUER="https://token.actions.githubusercontent.com"'
    );
    expect(script).not.toContain("resolve_latest_release_tag()");
    expect(script).not.toContain("releases/latest");
    expect(script).toContain('"$COSIGN_BIN" verify-blob "$CHECKSUM_PATH"');
    expect(script).toContain('--bundle "$BUNDLE_PATH"');
    expect(script).toContain('--certificate-identity-regexp "$COSIGN_CERT_IDENTITY_REGEXP"');
    expect(script).toContain('--certificate-oidc-issuer "$COSIGN_CERT_OIDC_ISSUER"');
    expect(script).toContain('CHECKSUM_CMD="sha256sum"');
    expect(script).toContain('CHECKSUM_CMD="shasum"');
    expect(script).toContain("shasum -a 256 -c - >/dev/null");
    expect(script).toContain("trap cleanup EXIT INT TERM");
    expect(script).toContain('TMP_TARGET="${INSTALL_DIR}/.compozy.tmp.$$"');
    expect(script).toContain('chmod 0755 "$TMP_TARGET"');
    expect(script).toContain('mv "$TMP_TARGET" "$TARGET"');
    expect(script).toContain('"$TARGET" version >/dev/null');
    expect(script).toContain('"$TARGET" install </dev/tty >/dev/tty');
    expect(downloadIndex).toBeGreaterThan(-1);
    expect(checksumDownloadIndex).toBeGreaterThan(downloadIndex);
    expect(bundleDownloadIndex).toBeGreaterThan(checksumDownloadIndex);
    expect(provenanceVerifyIndex).toBeGreaterThan(bundleDownloadIndex);
    expect(checksumVerifyIndex).toBeGreaterThan(provenanceVerifyIndex);
    expect(extractIndex).toBeGreaterThan(checksumVerifyIndex);
  });

  it("renders the template with an explicit tag and rejects unsafe tags", () => {
    const template = readInstallTemplate();
    const rendered = renderInstallScript(template, "v1.2.3-beta.4");

    expect(rendered).toContain('VERSION="${COMPOZY_VERSION:-v1.2.3-beta.4}"');
    expect(rendered).toContain("Install a specific release tag instead of v1.2.3-beta.4.");
    expect(rendered).not.toContain(INSTALL_VERSION_PLACEHOLDER);

    for (const tag of ["latest", "1.2.3", "v1.2", "v1.2.3;echo pwned", "v1.2.3 --flag", ""]) {
      expect(() => renderInstallScript(template, tag), tag).toThrow(
        /release tag must be a v-prefixed semver tag/
      );
    }
    expect(() => renderInstallScript("#!/bin/sh\n", "v1.2.3")).toThrow(/missing placeholder/);
  });

  it("renders identical scripts through the TypeScript and shell renderers", () => {
    const root = mkdtempSync(resolve(tmpdir(), "compozy-install-render-parity-"));
    try {
      const outputPath = join(root, "install.sh");
      const result = spawnSync("bash", [renderScriptPath, resolvedRouteTag, outputPath], {
        encoding: "utf8",
      });

      expect(result.status, result.stderr || result.stdout).toBe(0);
      expect(readFileSync(outputPath, "utf8")).toBe(
        renderInstallScript(readInstallTemplate(), resolvedRouteTag)
      );
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it("serves the rendered installer with script-safe headers from /install.sh", async () => {
    const response = await getInstallScriptRoute();
    const body = await response.text();

    expect(response.status).toBe(200);
    expect(response.headers.get("Content-Type")).toBe("text/plain; charset=utf-8");
    expect(response.headers.get("Cache-Control")).toBe("public, max-age=300, must-revalidate");
    expect(response.headers.get("X-Content-Type-Options")).toBe("nosniff");
    expect(body).toBe(renderInstallScript(INSTALL_SCRIPT_TEMPLATE, resolvedRouteTag));
    expect(body).toContain(`VERSION="\${COMPOZY_VERSION:-${resolvedRouteTag}}"`);
    expect(body).not.toContain(INSTALL_VERSION_PLACEHOLDER);
    expect(body).not.toContain("releases/latest");
  });

  it("keeps documented installer options executable in dry-run mode", () => {
    const help = runInstallScript(["--help"]);
    const defaultDryRun = runInstallScript(["--dry-run", "--skip-bootstrap"]);
    const pinnedDryRun = runInstallScript(["--dry-run", "--skip-bootstrap", "--version", "v0.1.0"]);
    const badOption = runInstallScript(["--not-a-real-option"]);

    expect(help.status).toBe(0);
    expect(help.stdout).toContain(VERIFIED_INSTALLER_COMMAND);
    expect(help.stdout).toContain(`instead of ${renderedTestTag}.`);
    for (const option of installOptions) {
      expect(help.stdout, option).toContain(option);
    }
    for (const envVar of installEnvVars) {
      expect(help.stdout, envVar).toContain(envVar);
    }

    expect(defaultDryRun.status).toBe(0);
    expect(defaultDryRun.stdout).toContain(`release: compozy/compozy ${renderedTestTag}`);
    expect(defaultDryRun.stdout).toContain("dry run complete");

    expect(pinnedDryRun.status).toBe(0);
    expect(pinnedDryRun.stdout).toContain("CompozyOS installer");
    expect(pinnedDryRun.stdout).toContain("release: compozy/compozy v0.1.0");
    expect(pinnedDryRun.stdout).toContain(
      "archive: https://github.com/compozy/compozy/releases/download/v0.1.0/"
    );
    expect(pinnedDryRun.stdout).toContain("bootstrap: skipped");
    expect(pinnedDryRun.stdout).toContain("dry run complete");
    expect(pinnedDryRun.stderr).toBe("");

    expect(badOption.status).not.toBe(0);
    expect(badOption.stderr).toContain("unknown option: --not-a-real-option");
  });

  it("keeps public installer release guarantees and recovery text in source", () => {
    const script = readInstallTemplate();

    for (const snippet of installerReleaseGuaranteeSnippets) {
      expect(script, snippet).toContain(snippet);
    }
    for (const snippet of installerCriticalErrorSnippets) {
      expect(script, snippet).toContain(snippet);
    }
    expect(script).toContain("printf 'compozy installer: %s\\n' \"$*\" >&2");
    expect(script).toContain('curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_PATH"');
    expect(script).toContain('curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_PATH"');
    expect(script).toContain('curl -fsSL "$BUNDLE_URL" -o "$BUNDLE_PATH"');
    expect(script).toContain(
      'printf \'%s\\n\' "$CHECKSUM_LINE" | (cd "$TMP_DIR" && sha256sum -c - >/dev/null)'
    );
    expect(script).toContain(
      'printf \'%s\\n\' "$CHECKSUM_LINE" | (cd "$TMP_DIR" && shasum -a 256 -c - >/dev/null)'
    );
    expect(script).toContain('log "next: compozy install"');
  });

  it("bootstraps a pinned temporary cosign verifier when none is on PATH", async () => {
    const root = mkdtempSync(resolve(tmpdir(), "compozy-install-cosign-bootstrap-"));
    try {
      const platform = currentInstallPlatform();
      const installDir = join(root, "bin");
      const cosignArgsPath = join(root, "cosign-args.txt");
      const archiveBody = createFixtureArchive(root);
      const cosignBody = '#!/bin/sh\nprintf \'%s\\n\' "$@" > "' + cosignArgsPath + '"\nexit 0\n';
      const routes = new Map<string, Buffer | string>([
        [platform.archiveName, archiveBody],
        ["checksums.txt", sha256(archiveBody) + "  " + platform.archiveName + "\n"],
        [
          "checksums.txt.sigstore.json",
          '{"mediaType":"application/vnd.dev.sigstore.bundle+json"}\n',
        ],
        [platform.cosignName, cosignBody],
      ]);

      await withFixtureServer(routes, async baseURL => {
        const script = renderInstallScript(readInstallTemplate(), renderedTestTag)
          .replace(
            'COSIGN_BASE_URL="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}"',
            'COSIGN_BASE_URL="' + baseURL + "/cosign/" + cosignVersion + '"'
          )
          .replace(platform.cosignDigest, sha256(cosignBody))
          .replace(
            'BASE_URL="https://github.com/${RELEASE_REPO}/releases/download/${VERSION}"',
            'BASE_URL="' + baseURL + '/releases/download/${VERSION}"'
          );
        const scriptPath = join(root, "install.sh");
        writeFileSync(scriptPath, script, "utf8");
        chmodSync(scriptPath, 0o755);

        const result = await runInstallScriptAsync(
          scriptPath,
          ["--version", "v9.9.9", "--skip-bootstrap", "--dir", installDir],
          {
            HOME: root,
            PATH: "/usr/bin:/bin",
            TZ: "UTC",
            LANG: "C.UTF-8",
            LC_ALL: "C.UTF-8",
            LC_CTYPE: "C.UTF-8",
            NODE_ENV: "test",
          }
        );

        expect(result.status, result.stderr || result.stdout).toBe(0);
        expect(result.stdout).toContain("downloading pinned cosign verifier " + cosignVersion);
        expect(readFileSync(cosignArgsPath, "utf8")).toContain("verify-blob");
        expect(readFileSync(join(installDir, "compozy"), "utf8")).toContain('"${1:-}" = "version"');
      });
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it.each([
    { label: "compatible v3", version: "v3.1.3", versionStatus: 0, bootstraps: false },
    { label: "incompatible v2", version: "v2.6.2", versionStatus: 0, bootstraps: true },
    { label: "unreadable version", version: "", versionStatus: 1, bootstraps: true },
  ])("selects the trusted verifier for a $label local cosign", async testCase => {
    const root = mkdtempSync(resolve(tmpdir(), "compozy-install-local-cosign-"));
    try {
      const localArgsPath = join(root, "local-cosign-args.txt");
      const localCosignBody = `#!/bin/sh
if [ "\${1:-}" = "version" ]; then
  printf 'GitVersion: ${testCase.version}\\n'
  exit ${testCase.versionStatus}
fi
printf '%s\\n' "$@" > "${localArgsPath}"
exit 0
`;
      const { downloadedArgsPath, result } = await runFixtureInstallWithLocalCosign(
        root,
        localCosignBody
      );

      expect(result.status, result.stderr || result.stdout).toBe(0);
      expect(existsSync(localArgsPath)).toBe(!testCase.bootstraps);
      expect(existsSync(downloadedArgsPath)).toBe(testCase.bootstraps);
      if (testCase.bootstraps) {
        expect(result.stdout).toContain("downloading pinned cosign verifier " + cosignVersion);
      } else {
        expect(result.stdout).toContain("using compatible local cosign v3");
      }
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it("fails provenance verification without falling back from compatible local cosign v3", async () => {
    const root = mkdtempSync(resolve(tmpdir(), "compozy-install-cosign-rejection-"));
    try {
      const localArgsPath = join(root, "local-cosign-args.txt");
      const localCosignBody = `#!/bin/sh
if [ "\${1:-}" = "version" ]; then
  printf 'GitVersion: v3.1.3\\n'
  exit 0
fi
printf '%s\\n' "$@" > "${localArgsPath}"
exit 23
`;
      const { downloadedArgsPath, result } = await runFixtureInstallWithLocalCosign(
        root,
        localCosignBody
      );

      expect(result.status).toBe(23);
      expect(existsSync(localArgsPath)).toBe(true);
      expect(existsSync(downloadedArgsPath)).toBe(false);
      expect(result.stdout).not.toContain("downloading pinned cosign verifier");
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it("runs install contract checks with a hermetic release environment", () => {
    const env = hermeticInstallEnv({
      COMPOZY_VERSION: "v9.9.9",
      COMPOZY_INSTALL_DIR: "/operator/bin",
      HOME: "/Users/operator",
      OPENAI_API_KEY: "sk-operator",
      PATH: "/usr/bin",
      PROVIDER_HOME: "/Users/operator/.provider",
      TZ: "America/Sao_Paulo",
    });

    expect(env.HOME).toBe("/Users/operator");
    expect(env.PATH).toBe("/usr/bin");
    expect(env.COMPOZY_VERSION).toBeUndefined();
    expect(env.COMPOZY_INSTALL_DIR).toBeUndefined();
    expect(env.OPENAI_API_KEY).toBeUndefined();
    expect(env.PROVIDER_HOME).toBeUndefined();
    expect(env.COMPOZY_SKIP_BOOTSTRAP).toBe("");
    expect(env.TZ).toBe("UTC");
    expect(env.LANG).toBe("C.UTF-8");
    expect(env.LC_ALL).toBe("C.UTF-8");
  });

  it("opens the tty before starting interactive bootstrap", () => {
    const script = readInstallTemplate();

    expect(script).not.toMatch(ttyPermissionProbePattern);
    expect(script).toMatch(ttyOpenProbePattern);
  });

  it("keeps hand-maintained install surfaces free of pinned prerelease literals", () => {
    const guardedPaths = [
      installTemplatePath,
      readmePath,
      rootMigrationGuidePath,
      installPagePath,
      migrationPagePath,
      landingInstallPath,
      homebrewNoticePath,
      releaseHeaderPath,
      releaseFooterPath,
    ];

    const offenders = guardedPaths
      .filter(path => pinnedBetaLiteralPattern.test(readSiteFile(path)))
      .map(path => relative(repoRoot, path));
    expect(offenders).toEqual([]);
  });

  it("keeps living install surfaces on the dynamic release contract", () => {
    const installPage = readSiteFile(installPagePath);
    const launchPost = readSiteFile(launchPostPath);
    const readme = readSiteFile(readmePath);
    const landing = readSiteFile(landingInstallPath);

    for (const command of [VERIFIED_INSTALLER_COMMAND, NPM_INSTALL_COMMAND]) {
      const missingPrimaryCommand = [readmePath, landingInstallPath, installPagePath]
        .filter(path => !readSiteFile(path).includes(command))
        .map(path => relative(repoRoot, path));
      expect(missingPrimaryCommand, command).toEqual([]);
    }

    for (const method of ["curl-pinned", "curl-env", "go"]) {
      expect(installPage, method).toContain(`<InstallCommand method="${method}" />`);
    }
    expect(installPage).toContain("<CurrentReleaseTag />");
    expect(installPage).toContain(sourceInstallCommand);
    expect(installPage).toContain("pinned temporary cosign verifier");
    expect(installPage).toContain("checksums.txt.sigstore.json");
    // --version/--dir and the env-var trio reach readers through the generated
    // commands; the page keeps the static flag examples that carry no tag.
    for (const option of ["--skip-bootstrap", "--dry-run"]) {
      expect(installPage, option).toContain(option);
    }
    expect(curlPinnedInstallCommand(resolvedRouteTag)).toContain(`--version ${resolvedRouteTag}`);
    expect(curlPinnedInstallCommand(resolvedRouteTag)).toContain("--dir");
    for (const envVar of installEnvVars) {
      expect(curlEnvInstallCommand(resolvedRouteTag), envVar).toContain(envVar);
    }
    expect(curlEnvInstallCommand(resolvedRouteTag)).toContain(
      `COMPOZY_VERSION=${resolvedRouteTag}`
    );
    expect(goInstallCommand(resolvedRouteTag)).toBe(
      `go install github.com/compozy/compozy@${resolvedRouteTag}`
    );

    expect(readme).toContain("go install github.com/compozy/compozy@");
    expect(readme).toContain(latestReleaseUrl);
    expect(readme).toContain(firstSessionCommand);
    expect(landing).toContain(firstSessionCommand);
    for (const guidePath of [rootMigrationGuidePath, migrationPagePath]) {
      expect(readSiteFile(guidePath)).toContain(latestReleaseUrl);
    }
    expect(launchPost).toContain(`records ${historicalLaunchBeta}`);
    expect(launchPost).toContain("[installation guide](/docs/getting-started/installation)");
  });

  it("keeps release collateral on the CompozyOS beta identity", () => {
    const readme = readSiteFile(readmePath);
    const header = readSiteFile(releaseHeaderPath);
    const footer = readSiteFile(releaseFooterPath);
    const cliff = readSiteFile(cliffPath);

    for (const content of [readme, header]) {
      expect(content).toContain(NPM_INSTALL_COMMAND);
      expect(content).toContain("go install github.com/compozy/compozy@");
      expect(content).toContain(VERIFIED_INSTALLER_COMMAND);
      expect(content).not.toContain("brew install");
    }
    expect(header).toContain(goInstallCommand("{{ .Tag }}"));
    expect(footer).toContain("github\\.com/compozy/compozy/");
    expect(cliff).toContain("github.com/compozy/compozy/compare/");
  });
});
