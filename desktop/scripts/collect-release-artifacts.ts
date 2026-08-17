import { copyFile, mkdir, readdir } from "node:fs/promises";
import { resolve } from "node:path";

import { validateReleaseConfig } from "../src/release/release-config";
import {
  parseReleaseArch,
  parseReleasePlatform,
  parseReleaseScriptFlags,
  requiredReleaseFlag,
} from "../src/release/release-script-flags";

const values = parseReleaseScriptFlags(
  process.argv.slice(2),
  ["platform", "arch", "version", "source", "output"],
  ["platform", "arch", "version"]
);
const platform = parseReleasePlatform(values.get("platform"));
const arch = parseReleaseArch(values.get("arch"));
const version = requiredReleaseFlag(values, "version");
const source = resolve(values.get("source") ?? ".artifacts/builder");
const output = resolve(values.get("output") ?? ".artifacts/release");
validateReleaseConfig({ arch, channel: "beta", notarize: false, platform, version });

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
