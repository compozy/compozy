import { createHash } from "node:crypto";
import { access, mkdir, readFile, readdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";

interface PackageMetadata {
  readonly version: string;
}

const desktopRoot = join(import.meta.dir, "..");
const packageMetadata = JSON.parse(
  await readFile(join(desktopRoot, "package.json"), "utf8")
) as PackageMetadata;
const match = /^(\d+)\.(\d+)\.(\d+)$/u.exec(packageMetadata.version);
if (!match?.[1] || !match[2] || !match[3]) {
  throw new Error("Desktop E2E updates require a stable semantic package version.");
}
const nextVersion = `${match[1]}.${match[2]}.${Number(match[3]) + 1}`;
const updateOutput = join(desktopRoot, ".artifacts", "e2e-update");
const baselineOutput = join(desktopRoot, ".artifacts", "e2e-baseline");
// Squirrel keys its native cache and helper by bundle ID, independently of updaterCacheDirName.
const appId = `com.compozy.os.e2e.${createHash("sha256").update(desktopRoot).digest("hex").slice(0, 16)}`;

async function buildInstrumentedMain(): Promise<void> {
  const build = Bun.spawn(["bun", "run", "build:main"], {
    cwd: desktopRoot,
    env: { ...process.env, COMPOZY_DESKTOP_E2E_BUILD: "1" },
    stdout: "inherit",
    stderr: "inherit",
  });
  const exitCode = await build.exited;
  if (exitCode !== 0) throw new Error(`E2E main-process build exited with ${exitCode}.`);
}

async function buildRuntime(version: string): Promise<void> {
  const build = Bun.spawn(["bun", "run", "build:runtime"], {
    cwd: desktopRoot,
    env: { ...process.env, COMPOZY_RELEASE_VERSION: version, COMPOZY_MIN_APP_VERSION: version },
    stdout: "inherit",
    stderr: "inherit",
  });
  const exitCode = await build.exited;
  if (exitCode !== 0) throw new Error(`E2E runtime ${version} build exited with ${exitCode}.`);
}

async function packageTarget(output: string, version: string): Promise<void> {
  await rm(output, { recursive: true, force: true });
  await mkdir(output, { recursive: true });
  const generatedConfig = join(output, "builder.json");
  const target = process.platform === "darwin" ? ["zip"] : ["AppImage"];
  await writeFile(
    generatedConfig,
    `${JSON.stringify(
      {
        extends: join(desktopRoot, "electron-builder.yml"),
        appId,
        directories: { output },
        electronFuses: { enableNodeCliInspectArguments: true },
        extraMetadata: { version },
        ...(process.platform === "darwin"
          ? { afterSign: join(desktopRoot, "scripts", "e2e-after-sign.cjs") }
          : {}),
        ...(process.platform === "darwin"
          ? { mac: { target, identity: "-", hardenedRuntime: false } }
          : { linux: { target } }),
      },
      null,
      2
    )}\n`,
    { mode: 0o600 }
  );
  const platformTarget = process.platform === "darwin" ? ["--mac", "zip"] : ["--linux", "AppImage"];
  const build = Bun.spawn(
    [
      "bunx",
      "electron-builder",
      "--config",
      generatedConfig,
      ...platformTarget,
      "--publish",
      "never",
    ],
    { cwd: desktopRoot, stdout: "inherit", stderr: "inherit" }
  );
  const exitCode = await build.exited;
  if (exitCode !== 0) throw new Error(`E2E update package build exited with ${exitCode}.`);
}

if (process.platform !== "darwin" && process.platform !== "linux") {
  throw new Error("Desktop E2E updates are supported on macOS and Linux.");
}
await buildInstrumentedMain();
await buildRuntime(packageMetadata.version);
await packageTarget(baselineOutput, packageMetadata.version);
try {
  await buildRuntime(nextVersion);
  await packageTarget(updateOutput, nextVersion);
} finally {
  // Other desktop scenarios use the baseline runtime from this shared build directory.
  await buildRuntime(packageMetadata.version);
}

const updateFiles = await readdir(updateOutput);
const assetSuffix = process.platform === "darwin" ? ".zip" : ".AppImage";
const manifestName = updateFiles.find(name => name.startsWith("latest") && name.endsWith(".yml"));
const assetName = updateFiles.find(name => name.endsWith(assetSuffix));
if (!manifestName || !assetName) throw new Error("The E2E updater fixture is incomplete.");
let baselineAppImage: string | undefined;
let baselineExecutable: string | undefined;
if (process.platform === "linux") {
  const baselineName = (await readdir(baselineOutput)).find(name => name.endsWith(".AppImage"));
  if (!baselineName) throw new Error("The E2E baseline AppImage is missing.");
  baselineAppImage = join(baselineOutput, baselineName);
} else {
  baselineExecutable = join(
    baselineOutput,
    `mac-${process.arch}`,
    "CompozyOS.app",
    "Contents",
    "MacOS",
    "CompozyOS"
  );
  await access(baselineExecutable);
}
await writeFile(
  join(desktopRoot, ".artifacts", "e2e-update-fixture.json"),
  `${JSON.stringify(
    {
      current_version: packageMetadata.version,
      next_version: nextVersion,
      directory: updateOutput,
      manifest: manifestName,
      asset: assetName,
      ...(baselineAppImage ? { baseline_app_image: baselineAppImage } : {}),
      ...(baselineExecutable ? { baseline_executable: baselineExecutable } : {}),
    },
    null,
    2
  )}\n`,
  { mode: 0o600 }
);
