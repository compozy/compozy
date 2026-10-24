import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";

import { buildReleaseConfig } from "../src/release/release-config";
import {
  parseBooleanFlag,
  parseReleaseArch,
  parseReleaseChannel,
  parseReleasePlatform,
  parseReleaseScriptFlags,
  requiredReleaseFlag,
} from "../src/release/release-script-flags";

const values = parseReleaseScriptFlags(
  process.argv.slice(2),
  ["arch", "channel", "notarize", "output", "platform", "version"],
  ["arch", "channel", "notarize", "platform", "version"]
);
const output = resolve(values.get("output") ?? ".artifacts/release-config.json");
const config = buildReleaseConfig({
  arch: parseReleaseArch(values.get("arch")),
  channel: parseReleaseChannel(values.get("channel")),
  notarize: parseBooleanFlag(values.get("notarize"), "notarize"),
  platform: parseReleasePlatform(values.get("platform")),
  version: requiredReleaseFlag(values, "version"),
});
await mkdir(dirname(output), { recursive: true });
await writeFile(output, `${JSON.stringify(config, null, 2)}\n`, { mode: 0o600 });
