const { execFile } = require("node:child_process");
const { promisify } = require("node:util");
const { join } = require("node:path");

const run = promisify(execFile);

module.exports = async function e2eAfterSign(context) {
  if (context.electronPlatformName !== "darwin") return;
  const appPath = join(context.appOutDir, `${context.packager.appInfo.productFilename}.app`);
  await run("codesign", [
    "--force",
    "--sign",
    "-",
    "--requirements",
    '=designated => identifier "com.compozy.os"',
    appPath,
  ]);
  await run("codesign", ["--verify", "--deep", "--strict", appPath]);
};
