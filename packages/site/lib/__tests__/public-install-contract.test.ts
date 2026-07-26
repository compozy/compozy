import { createHash } from "node:crypto";
import { chmodSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { tmpdir } from "node:os";
import { basename, join, relative, resolve } from "node:path";
import { spawn, spawnSync } from "node:child_process";
import { describe, expect, it } from "vitest";
import { siteRoot } from "../content-test-utils";

const publicRoot = resolve(siteRoot, "public");
const installScriptPath = resolve(publicRoot, "install.sh");
const headersPath = resolve(publicRoot, "_headers");
const installPagePath = resolve(siteRoot, "content/runtime/core/getting-started/installation.mdx");
const launchPostPath = resolve(
  siteRoot,
  "content/blog/posts/introducing-agh-the-first-agent-network-protocol.mdx"
);
const landingInstallPath = resolve(siteRoot, "components/landing/install-section.tsx");
const readmePath = resolve(siteRoot, "../../README.md");

const homebrewInstallCommand = "brew install compozy/compozy/agh";
const npmInstallCommand = "npm install -g @compozy/agh";
const goInstallCommand = "go install github.com/compozy/agh@latest";
const verifiedInstallerCommand = "curl -fsSL https://agh.network/install.sh | sh";
const sourceInstallCommand = "go build -o ./bin/agh .";
const workspaceAddCommand = 'agh workspace add "$PWD" --name current';
const firstSessionCommand = "agh session new --workspace current --agent general";
const retiredPackageInstallCommands = [
  "brew install --cask pedronauck/agh/agh",
  "pedronauck/agh/agh",
  "homebrew-agh",
];
const installOptions = ["--version", "--dir", "--skip-bootstrap", "--dry-run", "--help"];
const installEnvVars = ["AGH_VERSION", "AGH_INSTALL_DIR", "AGH_SKIP_BOOTSTRAP"];
const cosignVersion = "v2.2.4";
const cosignDigests = {
  "darwin/x64": "0e5a77a86115e4c00ba4243db01abceacb13cc06981c45e53ee71f2e1db8ce25",
  "darwin/arm64": "fcd310e64ecddc1eaa13fe814ac1c9fc02f6f9eacd9a58480ab8160eb8ca381e",
  "linux/x64": "97a6a1e15668a75fc4ff7a4dc4cb2f098f929cbea2f12faa9de31db6b42b17d7",
  "linux/arm64": "658087351e1d4f9c396b5f59ee5437461c06128f4ce80ba899ccaa1c0b6a8a62",
} as const;
const installerReleaseGuaranteeSnippets = [
  verifiedInstallerCommand,
  "Requires:",
  "curl, tar, and sha256sum or shasum.",
  "Uses local cosign when available; otherwise downloads a pinned temporary cosign verifier.",
  'COSIGN_VERSION="v2.2.4"',
  'COSIGN_BASE_URL="https://github.com/sigstore/cosign/releases/download/${COSIGN_VERSION}"',
  "cosign-darwin-amd64",
  "cosign-darwin-arm64",
  "cosign-linux-amd64",
  "cosign-linux-arm64",
  ...Object.values(cosignDigests),
  "resolve_cosign()",
  'COSIGN_BIN="$(command -v cosign)"',
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
  "failed to resolve latest release",
  "latest release resolved to unexpected ref:",
  "unsupported operating system:",
  "unsupported architecture:",
  "unsupported cosign verifier platform:",
  "curl is required",
  "tar is required",
  "checksum mismatch",
  "sha256sum or shasum is required to verify the download",
  "checksums.txt does not include",
  "archive did not contain an agh binary",
  "warning: ${INSTALL_DIR} is not on PATH",
  "add it to PATH or run ${TARGET} directly",
  "no interactive terminal detected; run this next:",
  "agh install",
];
const ttyPermissionProbePattern =
  /\[\s*-[rwe]\s+["']?\/dev\/tty["']?\s*\]|\btest\s+-[rwe]\s+["']?\/dev\/tty["']?/;
const ttyOpenProbePattern =
  /^\s*(?:if|elif)\s+!?\s*[^\n]*<\s*\/dev\/tty[^\n]*>\s*\/dev\/tty[^\n]*;\s*then/m;
const credentialEnvPattern =
  /(?:API_?KEY|ACCESS_KEY|PRIVATE_KEY|TOKEN|SECRET|PASSWORD|PASSWD|CREDENTIAL|AUTH|COOKIE|SESSION)|_KEY$/i;
type InstallEnv = Record<string, string | undefined>;

function readSiteFile(path: string): string {
  return readFileSync(path, "utf8");
}

function runInstallScript(args: string[]) {
  const installDir = mkdtempSync(resolve(tmpdir(), "agh-install-contract-"));
  return spawnSync("sh", [installScriptPath, ...args, "--dir", installDir], {
    cwd: siteRoot,
    encoding: "utf8",
    env: hermeticInstallEnv(),
  });
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
    archiveName: "agh_" + os + "_" + archiveArch + ".tar.gz",
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
  const binaryPath = join(sourceDir, "agh");
  writeFileSync(
    binaryPath,
    '#!/bin/sh\nif [ "${1:-}" = "version" ]; then exit 0; fi\nexit 0\n',
    "utf8"
  );
  chmodSync(binaryPath, 0o755);

  const archivePath = join(root, "agh.tar.gz");
  const result = spawnSync("tar", ["-czf", archivePath, "-C", sourceDir, "agh"], {
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
  env.AGH_SKIP_BOOTSTRAP = "";
  return env as NodeJS.ProcessEnv;
}

function blocksHermeticInstallEnv(key: string): boolean {
  const normalized = key.trim().toUpperCase();
  return (
    normalized.startsWith("AGH_") ||
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
  it("keeps the install script safe for the public curl entrypoint", () => {
    const script = readSiteFile(installScriptPath);
    const downloadIndex = script.indexOf('curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_PATH"');
    const checksumDownloadIndex = script.indexOf('curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_PATH"');
    const bundleDownloadIndex = script.indexOf('curl -fsSL "$BUNDLE_URL" -o "$BUNDLE_PATH"');
    const provenanceVerifyIndex = script.indexOf("verifying checksum provenance");
    const checksumVerifyIndex = script.indexOf('log "verifying checksum"');
    const extractIndex = script.indexOf('tar -xzf "$ARCHIVE_PATH" -C "$EXTRACT_DIR"');

    expect(script.startsWith("#!/bin/sh\nset -eu\n")).toBe(true);
    expect(script).toContain('RELEASE_REPO="compozy/agh"');
    expect(script).not.toContain("AGH_RELEASE_REPO");
    expect(script).toContain('BASE_URL="https://github.com/${RELEASE_REPO}/releases');
    expect(script).not.toContain("http://");
    expect(script).toContain('command -v curl >/dev/null 2>&1 || fail "curl is required"');
    expect(script).toContain('command -v tar >/dev/null 2>&1 || fail "tar is required"');
    expect(script).not.toContain('fail "cosign is required to verify release provenance"');
    expect(script).toContain('COSIGN_VERSION="v2.2.4"');
    expect(script).toContain("resolve_cosign()");
    expect(script).toContain("if command -v cosign >/dev/null 2>&1; then");
    expect(script).toContain('BUNDLE_URL="${BASE_URL}/checksums.txt.sigstore.json"');
    expect(script).toContain(
      "COSIGN_CERT_IDENTITY_REGEXP='^https://github\\.com/compozy/agh/\\.github/workflows/release\\.yml@refs/heads/main$'"
    );
    expect(script).toContain("v[0-9][A-Za-z0-9._-]*)");
    expect(script).toContain(
      'COSIGN_CERT_OIDC_ISSUER="https://token.actions.githubusercontent.com"'
    );
    expect(script).toContain("resolve_latest_release_tag()");
    expect(script).toContain('VERSION="$(resolve_latest_release_tag)"');
    expect(script).toContain('"$COSIGN_BIN" verify-blob "$CHECKSUM_PATH"');
    expect(script).toContain('--bundle "$BUNDLE_PATH"');
    expect(script).toContain('--certificate-identity-regexp "$COSIGN_CERT_IDENTITY_REGEXP"');
    expect(script).toContain('--certificate-oidc-issuer "$COSIGN_CERT_OIDC_ISSUER"');
    expect(script).toContain('CHECKSUM_CMD="sha256sum"');
    expect(script).toContain('CHECKSUM_CMD="shasum"');
    expect(script).toContain("shasum -a 256 -c - >/dev/null");
    expect(script).toContain("trap cleanup EXIT INT TERM");
    expect(script).toContain('TMP_TARGET="${INSTALL_DIR}/.agh.tmp.$$"');
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

  it("keeps documented installer options executable in dry-run mode", () => {
    const help = runInstallScript(["--help"]);
    const dryRun = runInstallScript(["--dry-run", "--skip-bootstrap", "--version", "v0.1.0"]);
    const badOption = runInstallScript(["--not-a-real-option"]);

    expect(help.status).toBe(0);
    expect(help.stdout).toContain(verifiedInstallerCommand);
    for (const option of installOptions) {
      expect(help.stdout, option).toContain(option);
    }
    for (const envVar of installEnvVars) {
      expect(help.stdout, envVar).toContain(envVar);
    }

    expect(dryRun.status).toBe(0);
    expect(dryRun.stdout).toContain("AGH installer");
    expect(dryRun.stdout).toContain("release: compozy/agh v0.1.0");
    expect(dryRun.stdout).toContain(
      "archive: https://github.com/compozy/agh/releases/download/v0.1.0/"
    );
    expect(dryRun.stdout).toContain("bootstrap: skipped");
    expect(dryRun.stdout).toContain("dry run complete");
    expect(dryRun.stderr).toBe("");

    expect(badOption.status).not.toBe(0);
    expect(badOption.stderr).toContain("unknown option: --not-a-real-option");
  });

  it("keeps public installer release guarantees and recovery text in source", () => {
    const script = readSiteFile(installScriptPath);

    for (const snippet of installerReleaseGuaranteeSnippets) {
      expect(script, snippet).toContain(snippet);
    }
    for (const snippet of installerCriticalErrorSnippets) {
      expect(script, snippet).toContain(snippet);
    }
    expect(script).toContain("printf 'agh installer: %s\\n' \"$*\" >&2");
    expect(script).toContain('curl -fsSL "$ARCHIVE_URL" -o "$ARCHIVE_PATH"');
    expect(script).toContain('curl -fsSL "$CHECKSUM_URL" -o "$CHECKSUM_PATH"');
    expect(script).toContain('curl -fsSL "$BUNDLE_URL" -o "$BUNDLE_PATH"');
    expect(script).toContain(
      'printf \'%s\\n\' "$CHECKSUM_LINE" | (cd "$TMP_DIR" && sha256sum -c - >/dev/null)'
    );
    expect(script).toContain(
      'printf \'%s\\n\' "$CHECKSUM_LINE" | (cd "$TMP_DIR" && shasum -a 256 -c - >/dev/null)'
    );
    expect(script).toContain('log "next: agh install"');
  });

  it("bootstraps a pinned temporary cosign verifier when none is on PATH", async () => {
    const root = mkdtempSync(resolve(tmpdir(), "agh-install-cosign-bootstrap-"));
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
        const script = readSiteFile(installScriptPath)
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
        expect(readFileSync(join(installDir, "agh"), "utf8")).toContain('"${1:-}" = "version"');
      });
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  });

  it("runs install contract checks with a hermetic release environment", () => {
    const env = hermeticInstallEnv({
      AGH_VERSION: "v9.9.9",
      AGH_INSTALL_DIR: "/operator/bin",
      HOME: "/Users/operator",
      OPENAI_API_KEY: "sk-operator",
      PATH: "/usr/bin",
      PROVIDER_HOME: "/Users/operator/.provider",
      TZ: "America/Sao_Paulo",
    });

    expect(env.HOME).toBe("/Users/operator");
    expect(env.PATH).toBe("/usr/bin");
    expect(env.AGH_VERSION).toBeUndefined();
    expect(env.AGH_INSTALL_DIR).toBeUndefined();
    expect(env.OPENAI_API_KEY).toBeUndefined();
    expect(env.PROVIDER_HOME).toBeUndefined();
    expect(env.AGH_SKIP_BOOTSTRAP).toBe("");
    expect(env.TZ).toBe("UTC");
    expect(env.LANG).toBe("C.UTF-8");
    expect(env.LC_ALL).toBe("C.UTF-8");
  });

  it("opens the tty before starting interactive bootstrap", () => {
    const script = readSiteFile(installScriptPath);

    expect(script).not.toMatch(ttyPermissionProbePattern);
    expect(script).toMatch(ttyOpenProbePattern);
  });

  it("serves install.sh with script-safe headers", () => {
    const headers = readSiteFile(headersPath);
    const installHeaderBlock = headers.match(/\/install\.sh\n([\s\S]*?)(?:\n\n|$)/)?.[1] ?? "";

    expect(installHeaderBlock).toContain("Content-Type: text/plain; charset=utf-8");
    expect(installHeaderBlock).toContain("Cache-Control: public, max-age=300, must-revalidate");
    expect(headers).toContain("X-Content-Type-Options: nosniff");
    expect(headers).toContain("Content-Security-Policy:");
  });

  it("keeps README, landing, docs, and launch post aligned on install commands", () => {
    const readme = readSiteFile(readmePath);
    const installPage = readSiteFile(installPagePath);
    const landingInstall = readSiteFile(landingInstallPath);
    const launchPost = readSiteFile(launchPostPath);

    for (const command of [homebrewInstallCommand, npmInstallCommand, goInstallCommand]) {
      const missingPrimaryCommand = [
        readmePath,
        landingInstallPath,
        installPagePath,
        launchPostPath,
      ]
        .filter(path => !readSiteFile(path).includes(command))
        .map(path => relative(siteRoot, path));
      expect(missingPrimaryCommand, command).toEqual([]);
    }

    expect(installPage).toContain(verifiedInstallerCommand);
    expect(installPage).toContain(sourceInstallCommand);
    expect(launchPost).toContain("agh install");
    for (const retiredCommand of retiredPackageInstallCommands) {
      expect(readme).not.toContain(retiredCommand);
      expect(landingInstall).not.toContain(retiredCommand);
      expect(installPage).not.toContain(retiredCommand);
      expect(launchPost).not.toContain(retiredCommand);
    }
    for (const command of [workspaceAddCommand, firstSessionCommand]) {
      const missingFirstSessionCommand = [readmePath, landingInstallPath, launchPostPath]
        .filter(path => !readSiteFile(path).includes(command))
        .map(path => relative(siteRoot, path));
      expect(missingFirstSessionCommand, command).toEqual([]);
    }
    for (const option of installOptions.slice(0, -1)) {
      expect(installPage, option).toContain(option);
    }
    for (const envVar of ["AGH_VERSION", "AGH_INSTALL_DIR", "AGH_SKIP_BOOTSTRAP"]) {
      expect(installPage, envVar).toContain(envVar);
    }
    expect(installPage).toContain("pinned temporary cosign verifier");
    expect(installPage).toContain("checksums.txt.sigstore.json");
  });
});
