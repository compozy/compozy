import { mkdir, readFile, writeFile } from "node:fs/promises";

import {
  mergeMacUpdateManifests,
  rewriteUpdateManifestURLs,
  selectUpdateManifestFiles,
} from "../src/release/mac-manifest";

const [arm64Path, x64Path, linuxPath, windowsPath, outputDirectory, downloadBase] =
  process.argv.slice(2);
if (!arm64Path || !x64Path || !linuxPath || !windowsPath || !outputDirectory || !downloadBase) {
  throw new Error(
    "usage: prepare-channel-manifests <arm64.yml> <x64.yml> <linux.yml> <windows.yml> <output-dir> <download-base>"
  );
}
const mac = mergeMacUpdateManifests(
  await readFile(arm64Path, "utf8"),
  await readFile(x64Path, "utf8")
);
const linux = selectUpdateManifestFiles(await readFile(linuxPath, "utf8"), ["-linux-x64.AppImage"]);
await mkdir(outputDirectory, { recursive: true });
await writeFile(
  `${outputDirectory}/latest-mac.yml`,
  rewriteUpdateManifestURLs(mac, downloadBase),
  "utf8"
);
await writeFile(
  `${outputDirectory}/latest-linux.yml`,
  rewriteUpdateManifestURLs(linux, downloadBase),
  "utf8"
);

const windows = selectUpdateManifestFiles(await readFile(windowsPath, "utf8"), ["-win-x64.exe"]);
await writeFile(
  `${outputDirectory}/latest.yml`,
  rewriteUpdateManifestURLs(windows, downloadBase),
  "utf8"
);
