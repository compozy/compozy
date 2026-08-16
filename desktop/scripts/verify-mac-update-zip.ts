import { spawnSync } from "node:child_process";
import { basename } from "node:path";

import { assertMacZipSymlinks } from "../src/release/mac-zip";

const zipPath = process.argv[2];
if (!zipPath) throw new Error("usage: verify-mac-update-zip <artifact.zip>");
const listing = spawnSync("zipinfo", ["-l", zipPath], { encoding: "utf8" });
if (listing.status !== 0) throw new Error(listing.stderr.trim() || "zipinfo failed");
const entries = new Map<string, string>();
let appBundle = "";
for (const line of listing.stdout.split(/\r?\n/u)) {
  const match = line.match(/^([dl-][rwx-]{9})\s+.*?\s([^\s].*)$/u);
  if (!match?.[1] || !match[2]) continue;
  entries.set(match[2], match[1]);
  const bundle = match[2].match(/^([^/]+\.app)\//u)?.[1];
  if (bundle) appBundle = bundle;
}
if (!appBundle) throw new Error(`${basename(zipPath)} contains no app bundle.`);
assertMacZipSymlinks(appBundle, entries);
