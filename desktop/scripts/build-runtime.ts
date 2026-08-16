import { createHash } from "node:crypto";
import { chmod, mkdir, readFile, writeFile } from "node:fs/promises";
import { join, resolve } from "node:path";

const desktopRoot = join(import.meta.dir, "..");
const repositoryRoot = resolve(desktopRoot, "..");
const output = join(desktopRoot, ".artifacts", "runtime");
const runtimeName = process.platform === "win32" ? "compozy.exe" : "compozy";
const runtimePath = join(output, runtimeName);
await mkdir(output, { recursive: true });

const packageMetadata: unknown = JSON.parse(
  await readFile(join(desktopRoot, "package.json"), "utf8")
);
if (
  !packageMetadata ||
  typeof packageMetadata !== "object" ||
  typeof Reflect.get(packageMetadata, "version") !== "string" ||
  !/^\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$/u.test(Reflect.get(packageMetadata, "version"))
) {
  throw new Error("The desktop package version must be a semantic version.");
}
const version = Reflect.get(packageMetadata, "version");

function gitMetadata(format: string): string {
  const result = Bun.spawnSync(["git", "show", "-s", `--format=${format}`, "HEAD"], {
    cwd: repositoryRoot,
    stdout: "pipe",
    stderr: "pipe",
  });
  if (result.exitCode !== 0) {
    throw new Error(result.stderr.toString().trim() || "Could not read the build commit metadata.");
  }
  const value = result.stdout.toString().trim();
  if (!value) throw new Error("The build commit metadata is empty.");
  return value;
}

const commit = process.env.COMPOZY_BUILD_COMMIT?.trim() || gitMetadata("%h");
const buildDate = process.env.COMPOZY_BUILD_DATE?.trim() || gitMetadata("%cI");
const ldflags = [
  `-X github.com/compozy/compozy/internal/version.Version=${version}`,
  `-X github.com/compozy/compozy/internal/version.Commit=${commit}`,
  `-X github.com/compozy/compozy/internal/version.BuildDate=${buildDate}`,
  `-X github.com/compozy/compozy/internal/version.MinAppVersion=${version}`,
].join(" ");

const build = Bun.spawn(
  ["go", "build", "-trimpath", "-ldflags", ldflags, "-o", runtimePath, "./cmd/compozy"],
  {
    cwd: repositoryRoot,
    stdout: "inherit",
    stderr: "inherit",
  }
);
const exitCode = await build.exited;
if (exitCode !== 0) throw new Error(`Bundled runtime build exited with status ${exitCode}.`);
if (process.platform !== "win32") await chmod(runtimePath, 0o700);
const payload = await readFile(runtimePath);
const manifest = {
  schema_version: 1,
  asset: runtimeName,
  sha256: createHash("sha256").update(payload).digest("hex"),
};
await writeFile(join(output, "runtime-manifest.json"), `${JSON.stringify(manifest, null, 2)}\n`, {
  encoding: "utf8",
  mode: 0o600,
});
