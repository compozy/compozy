import { spawnSync } from "node:child_process";
import { readFile, writeFile } from "node:fs/promises";

import { appleIDNotaryArguments } from "../src/release/notary-auth";
import { refreshUpdateManifestFileIntegrity } from "../src/release/mac-manifest";

const dmgPath = process.argv[2];
const manifestPath = process.argv[3];
if (!dmgPath || !manifestPath) {
  throw new Error("usage: finalize-macos-release <artifact.dmg> <latest-mac.yml>");
}

function run(command: string, args: readonly string[]): void {
  const result = spawnSync(command, [...args], { stdio: "inherit" });
  if (result.status !== 0) throw new Error(`${command} failed with exit code ${result.status}.`);
}

run("codesign", ["--verify", "--strict", "--verbose=4", dmgPath]);
run("xcrun", ["notarytool", "submit", dmgPath, ...appleIDNotaryArguments(process.env), "--wait"]);
run("xcrun", ["stapler", "staple", dmgPath]);
run("xcrun", ["stapler", "validate", dmgPath]);
run("spctl", [
  "--assess",
  "--type",
  "open",
  "--context",
  "context:primary-signature",
  "--verbose=4",
  dmgPath,
]);

const manifest = await readFile(manifestPath, "utf8");
const refreshedManifest = await refreshUpdateManifestFileIntegrity(manifest, dmgPath);
await writeFile(manifestPath, refreshedManifest, "utf8");
