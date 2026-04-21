import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

async function readJSON<T>(relativePath: string): Promise<T> {
  const filePath = path.join(rootDir, relativePath);
  const fileContents = await readFile(filePath, "utf8");

  return JSON.parse(fileContents) as T;
}

interface PackageJSON {
  workspaces?: string[];
  scripts?: Record<string, string>;
  dependencies?: Record<string, string>;
  exports?: Record<string, string>;
}

interface TsConfig {
  extends?: string;
  compilerOptions?: Record<string, unknown>;
  include?: string[];
  exclude?: string[];
}

describe("frontend workspace configuration", () => {
  it("keeps sdk workspaces while adding web and shared ui packages", async () => {
    const packageJSON = await readJSON<PackageJSON>("package.json");

    expect(packageJSON.workspaces).toEqual(expect.arrayContaining(["sdk/*", "packages/ui", "web"]));
    expect(packageJSON.workspaces).not.toContain("dev/*");
    expect(packageJSON.scripts?.build).toBe("turbo run build");
    expect(packageJSON.scripts?.typecheck).toBe("turbo run typecheck");
    expect(packageJSON.scripts?.dev).toBeUndefined();
    expect(packageJSON.scripts?.["dev:global"]).toBeUndefined();
    expect(packageJSON.scripts?.["dev:daemon"]).toBeUndefined();
  });

  it("wires the web package to the shared ui workspace and bundle contract", async () => {
    const packageJSON = await readJSON<PackageJSON>("web/package.json");
    const indexHTML = await readFile(path.join(rootDir, "web/index.html"), "utf8");
    const distPlaceholder = await readFile(path.join(rootDir, "web/dist/.keep"), "utf8");

    expect(packageJSON.dependencies?.["@compozy/ui"]).toBe("workspace:*");
    expect(packageJSON.scripts?.build).toContain("restore-dist-placeholder");
    expect(indexHTML).toContain('id="app"');
    expect(distPlaceholder).toContain("Tracked placeholder");
  });

  it("keeps both new workspaces on the repository strict tsconfig base", async () => {
    const webTSConfig = await readJSON<TsConfig>("web/tsconfig.json");
    const uiTSConfig = await readJSON<TsConfig>("packages/ui/tsconfig.json");
    const uiBuildTSConfig = await readJSON<TsConfig>("packages/ui/tsconfig.build.json");
    const uiPackage = await readJSON<PackageJSON>("packages/ui/package.json");

    expect(webTSConfig.extends).toBe("../tsconfig.base.json");
    expect(uiTSConfig.extends).toBe("../../tsconfig.base.json");
    expect(uiBuildTSConfig.extends).toBe("./tsconfig.json");
    expect(webTSConfig.compilerOptions?.strict).toBe(true);
    expect(uiTSConfig.compilerOptions?.strict).toBe(true);
    expect(uiBuildTSConfig.include).toEqual(["src/**/*.ts", "src/**/*.tsx"]);
    expect(uiBuildTSConfig.exclude).toEqual([
      "tests",
      ".storybook",
      "dist",
      "node_modules",
      "coverage",
    ]);
    expect(uiPackage.exports).toEqual({
      ".": "./src/index.ts",
      "./tokens.css": "./src/tokens.css",
      "./utils": "./src/lib/utils.ts",
    });
  });

  it("keeps a direct binary-based daemon entrypoint for the single-url hot reload flow", async () => {
    const daemonDevScript = await readFile(path.join(rootDir, "scripts/dev-web-proxy.sh"), "utf8");
    const viteConfig = await readFile(path.join(rootDir, "web/vite.config.ts"), "utf8");

    expect(daemonDevScript).toContain("./bin/compozy daemon start --foreground");
    expect(daemonDevScript).toContain("bun run --cwd web dev");
    expect(daemonDevScript).toContain("COMPOZY_WEB_DEV_PROXY");
    expect(daemonDevScript).toContain("COMPOZY_DAEMON_HTTP_PORT");
    expect(daemonDevScript).toContain("COMPOZY_DEV_HOME");
    expect(viteConfig).toContain('host: "127.0.0.1"');
    expect(viteConfig).toContain("port: 3000");
    expect(viteConfig).toContain("strictPort: true");
  });
});
