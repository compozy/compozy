import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";

import {
  buildReleaseConfig,
  type ReleaseArch,
  type ReleasePlatform,
} from "../src/release/release-config";

const values = new Map<string, string>();
for (let index = 2; index < process.argv.length; index += 2) {
  const key = process.argv[index];
  const value = process.argv[index + 1];
  if (!key?.startsWith("--") || !value) throw new Error("Release config flags require values.");
  values.set(key.slice(2), value);
}
const output = resolve(values.get("output") ?? ".artifacts/release-config.json");
const config = buildReleaseConfig({
  arch: values.get("arch") as ReleaseArch,
  channel: "beta",
  notarize: values.get("notarize") === "true",
  platform: values.get("platform") as ReleasePlatform,
  version: values.get("version") ?? "",
});
await mkdir(dirname(output), { recursive: true });
await writeFile(output, `${JSON.stringify(config, null, 2)}\n`, { mode: 0o600 });
