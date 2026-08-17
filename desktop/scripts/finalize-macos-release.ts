import { spawnSync } from "node:child_process";

import { appleIDNotaryArguments } from "../src/release/notary-auth";

const dmgPath = process.argv[2];
if (!dmgPath) throw new Error("usage: finalize-macos-release <artifact.dmg>");

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
