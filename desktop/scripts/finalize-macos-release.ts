import { spawnSync } from "node:child_process";

const dmgPath = process.argv[2];
if (!dmgPath) throw new Error("usage: finalize-macos-release <artifact.dmg>");

function required(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) throw new Error(`macOS release requires ${name}.`);
  return value;
}

function run(command: string, args: readonly string[]): void {
  const result = spawnSync(command, [...args], { encoding: "utf8", stdio: "inherit" });
  if (result.status !== 0) throw new Error(`${command} failed with exit code ${result.status}.`);
}

const key = required("APPLE_API_KEY");
const keyID = required("APPLE_API_KEY_ID");
const issuer = required("APPLE_API_ISSUER");
run("codesign", ["--verify", "--strict", "--verbose=4", dmgPath]);
run("xcrun", [
  "notarytool",
  "submit",
  dmgPath,
  "--key",
  key,
  "--key-id",
  keyID,
  "--issuer",
  issuer,
  "--wait",
]);
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
