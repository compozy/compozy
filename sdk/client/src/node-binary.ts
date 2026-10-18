import { constants } from "node:fs";
import { access, stat } from "node:fs/promises";
import { createRequire } from "node:module";
import { delimiter, dirname, join, resolve } from "node:path";

import { CompozyBinaryNotFoundError } from "./errors.js";

async function executable(path: string): Promise<boolean> {
  try {
    await access(path, process.platform === "win32" ? constants.F_OK : constants.X_OK);
    return (await stat(path)).isFile();
  } catch {
    return false;
  }
}

export async function resolveBinary(binaryPath?: string): Promise<string> {
  for (const candidate of [binaryPath, process.env.COMPOZY_BIN]) {
    if (candidate && (await executable(candidate))) return resolve(candidate);
  }
  const name = process.platform === "win32" ? "compozy.exe" : "compozy";
  // Node anchors argv[1] to the application entry point even when the caller
  // launches it from another directory. REPL/eval callers fall back to cwd.
  const appEntry = process.argv[1];
  const lookupPaths = [
    ...(appEntry ? [resolve(appEntry)] : []),
    join(process.cwd(), "package.json"),
  ];
  for (const lookupPath of lookupPaths) {
    try {
      const require = createRequire(lookupPath);
      const packagePath = require.resolve("@compozy/cli/package.json");
      const candidate = join(dirname(packagePath), "bin", name);
      if (await executable(candidate)) return candidate;
    } catch {
      // The CLI package is optional; PATH is the final discovery source.
    }
  }
  for (const directory of (process.env.PATH ?? "").split(delimiter)) {
    if (!directory) continue;
    const candidate = join(directory, name);
    if (await executable(candidate)) return resolve(candidate);
  }
  throw new CompozyBinaryNotFoundError(
    "compozy binary not found. Checked: binaryPath option, COMPOZY_BIN, @compozy/cli, PATH. " +
      "Install it with `npm install @compozy/cli` or set COMPOZY_BIN."
  );
}
