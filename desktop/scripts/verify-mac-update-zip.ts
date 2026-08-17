import { spawnSync } from "node:child_process";
import { basename } from "node:path";

import { assertMacZipSymlinks, parseZipInfoLongListing } from "../src/release/mac-zip";

const zipPath = process.argv[2];
if (!zipPath) throw new Error("usage: verify-mac-update-zip <artifact.zip>");
const listing = spawnSync("zipinfo", ["-l", zipPath], { encoding: "utf8" });
if (listing.status !== 0) throw new Error(listing.stderr.trim() || "zipinfo failed");
const entries = parseZipInfoLongListing(listing.stdout);
let appBundle = "";
for (const path of entries.keys()) {
  const bundle = path.match(/^([^/]+\.app)\//u)?.[1];
  if (bundle) appBundle = bundle;
}
if (!appBundle) throw new Error(`${basename(zipPath)} contains no app bundle.`);
assertMacZipSymlinks(appBundle, entries);
