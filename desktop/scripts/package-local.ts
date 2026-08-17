import { mkdir, rm, writeFile } from "node:fs/promises";
import { join } from "node:path";

const desktopRoot = join(import.meta.dir, "..");
const artifacts = join(desktopRoot, ".artifacts");
const configPath = join(artifacts, "local-builder.json");

await mkdir(artifacts, { recursive: true });
await writeFile(
  configPath,
  `${JSON.stringify(
    {
      extends: join(desktopRoot, "electron-builder.yml"),
      ...(process.platform === "darwin" ? { mac: { identity: "-", hardenedRuntime: false } } : {}),
    },
    null,
    2
  )}\n`,
  { mode: 0o600 }
);
try {
  const build = Bun.spawn(
    ["bunx", "electron-builder", "--config", configPath, "--dir", "--publish", "never"],
    { cwd: desktopRoot, stdout: "inherit", stderr: "inherit" }
  );
  const exitCode = await build.exited;
  if (exitCode !== 0) throw new Error(`Local desktop package build exited with ${exitCode}.`);
} finally {
  await rm(configPath, { force: true });
}
