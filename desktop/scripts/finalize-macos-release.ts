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

// Stapling fetches the fresh ticket from Apple's CloudKit service, which can drop the connection
// right after notarization is accepted. Stapling is idempotent, so retry it with a bounded backoff
// instead of failing the release and paying for another notarization.
const STAPLE_ATTEMPTS = 6;
const STAPLE_BACKOFF_MS = 30_000;

async function staple(path: string): Promise<void> {
  for (let attempt = 1; ; attempt += 1) {
    const result = spawnSync("xcrun", ["stapler", "staple", path], { stdio: "inherit" });
    if (result.status === 0) return;
    if (attempt >= STAPLE_ATTEMPTS) {
      throw new Error(
        `xcrun stapler staple failed after ${attempt} attempts (exit ${result.status}).`
      );
    }
    const delay = STAPLE_BACKOFF_MS * attempt;
    console.warn(
      `stapler staple attempt ${attempt} failed (exit ${result.status}); retrying in ${delay / 1000}s.`
    );
    await new Promise(resolve => setTimeout(resolve, delay));
  }
}

run("codesign", ["--verify", "--strict", "--verbose=4", dmgPath]);
run("xcrun", ["notarytool", "submit", dmgPath, ...appleIDNotaryArguments(process.env), "--wait"]);
await staple(dmgPath);
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
