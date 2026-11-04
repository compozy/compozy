const { createHash } = require("node:crypto");
const { readFile, writeFile } = require("node:fs/promises");
const { join } = require("node:path");

// Authenticode changes the runtime bytes; seal the signed binary before making the installer.
module.exports = async context => {
  if (context.electronPlatformName !== "win32") return;
  const root = join(context.appOutDir, "resources", "runtime");
  const manifestPath = join(root, "runtime-manifest.json");
  const manifest = JSON.parse(await readFile(manifestPath, "utf8"));
  if (manifest.schema_version !== 1 || manifest.asset !== "compozy.exe") {
    throw new Error("The Windows runtime integrity manifest is invalid.");
  }
  manifest.sha256 = createHash("sha256")
    .update(await readFile(join(root, "compozy.exe")))
    .digest("hex");
  await writeFile(manifestPath, `${JSON.stringify(manifest, null, 2)}\n`);
};
