import { copyFile, mkdir, readdir } from "node:fs/promises";
import { resolve } from "node:path";

type Platform = "linux" | "mac";
type Arch = "arm64" | "x64";

const values = new Map<string, string>();
for (let index = 2; index < process.argv.length; index += 2) {
  const key = process.argv[index];
  const value = process.argv[index + 1];
  if (!key?.startsWith("--") || !value)
    throw new Error("Artifact collection flags require values.");
  values.set(key.slice(2), value);
}

const platform = values.get("platform") as Platform;
const arch = values.get("arch") as Arch;
const version = values.get("version") ?? "";
const source = resolve(values.get("source") ?? ".artifacts/builder");
const output = resolve(values.get("output") ?? ".artifacts/release");
if (platform !== "mac" && platform !== "linux") throw new Error("Platform must be mac or linux.");
if (arch !== "arm64" && arch !== "x64") throw new Error("Architecture must be arm64 or x64.");
if (platform === "linux" && arch !== "x64")
  throw new Error("Linux release artifacts support x64 only.");

const packages =
  platform === "mac"
    ? [`CompozyOS-${version}-mac-${arch}.dmg`, `CompozyOS-${version}-mac-${arch}.zip`]
    : [`CompozyOS-${version}-linux-x64.AppImage`, `CompozyOS-${version}-linux-x64.deb`];
const files = await readdir(source);
const manifestCandidates = files.filter(file => /^latest.*\.ya?ml$/u.test(file));
if (manifestCandidates.length !== 1) {
  throw new Error(
    `Expected one updater manifest in ${source}, found ${manifestCandidates.length}.`
  );
}
for (const packageName of packages) {
  if (!files.includes(packageName)) throw new Error(`Missing release artifact ${packageName}.`);
}

await mkdir(output, { recursive: true });
for (const packageName of packages) {
  await copyFile(resolve(source, packageName), resolve(output, packageName));
}
const manifestName = platform === "mac" ? `latest-mac-${arch}.yml` : "latest-linux.yml";
await copyFile(resolve(source, manifestCandidates[0]!), resolve(output, manifestName));
