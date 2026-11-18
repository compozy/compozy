// UT-003 / UT-004: the export scripts fail before any capture when the dev server is unreachable
// or BRAND_EXPORT_DIR points into the git working tree (US-003.EC-1, US-003.AC-3). The scripts
// run for real; the only boundary faked is the network (a port with nothing listening).
import { spawnSync } from "node:child_process";
import { existsSync, mkdtempSync, realpathSync, rmSync } from "node:fs";
import { createServer } from "node:net";
import { homedir, tmpdir } from "node:os";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { afterEach, describe, expect, it } from "vitest";

import { REPO_ROOT, resolveExportDir, resolveExportFile } from "../../../scripts/export-env";

const script = (name: string) =>
  fileURLToPath(new URL(`../../../scripts/${name}.ts`, import.meta.url));
const dirs: string[] = [];
afterEach(() => {
  for (const d of dirs.splice(0)) rmSync(d, { recursive: true, force: true });
});

/** A localhost URL whose port has nothing listening. */
async function deadBaseUrl() {
  const server = createServer();
  await new Promise<void>(ok => server.listen(0, "127.0.0.1", ok));
  const address = server.address();
  const port = typeof address === "object" && address ? address.port : 0;
  await new Promise(ok => server.close(ok));
  return `http://127.0.0.1:${port}`;
}

function run(name: string, env: Record<string, string>, args: string[] = []) {
  return spawnSync("bun", [script(name), ...args], {
    encoding: "utf8",
    env: { ...process.env, ...env },
  });
}

describe("export scripts", () => {
  it.each(["export-kit", "export-visuals"])(
    "%s exits 1 naming the URL when the dev server is unreachable (UT-003)",
    async name => {
      const base = await deadBaseUrl();
      const out = mkdtempSync(join(tmpdir(), "brand-ut003-"));
      dirs.push(out);
      const result = run(name, { BRAND_BASE_URL: base, BRAND_EXPORT_DIR: out });
      expect(result.status).toBe(1);
      expect(result.stderr).toContain(
        `error: brand dev server not reachable at ${base} — run \`bun run dev\` first`
      );
    }
  );

  it("refuses a BRAND_EXPORT_DIR inside the git working tree before writing (UT-004)", () => {
    const inside = join(REPO_ROOT, "packages/brand/.ut004-exports");
    const result = run("export-kit", {
      BRAND_EXPORT_DIR: inside,
      BRAND_BASE_URL: "http://127.0.0.1:9",
    });
    expect(result.status).toBe(1);
    expect(result.stderr).toContain("is inside the git working tree");
    expect(existsSync(inside)).toBe(false);
  });

  it("resolves the default and ~ paths outside the repo and rejects the repo root itself (UT-004)", () => {
    expect(resolveExportDir({})).toBe(join(homedir(), "Dev/compozy/_rebranding/exports"));
    expect(resolveExportDir({ BRAND_EXPORT_DIR: "~/brand-out" })).toBe(
      join(homedir(), "brand-out")
    );
    expect(() => resolveExportDir({ BRAND_EXPORT_DIR: REPO_ROOT })).toThrow(
      /inside the git working tree/
    );
    expect(() =>
      resolveExportDir({ BRAND_EXPORT_DIR: join(REPO_ROOT, "web/../packages/x") })
    ).toThrow(/inside the git working tree/);
    expect(() => resolveExportDir({ BRAND_EXPORT_DIR: join(REPO_ROOT, "..exports") })).toThrow(
      /inside the git working tree/
    );
    expect(resolveExportDir({ BRAND_EXPORT_DIR: join(REPO_ROOT, "../brand-out") })).toBe(
      join(realpathSync(dirname(REPO_ROOT)), "brand-out")
    );
  });

  it("shot refuses a BRAND_EXPORT_DIR inside the git working tree before capturing (UT-004)", () => {
    const inside = join(REPO_ROOT, "packages/brand/.ut004-shots");
    const result = run("shot", { BRAND_EXPORT_DIR: inside }, ["http://127.0.0.1:9", "page.png"]);
    expect(result.status).toBe(1);
    expect(result.stderr).toContain("is inside the git working tree");
    expect(existsSync(inside)).toBe(false);
  });

  it("shot refuses a destination outside BRAND_EXPORT_DIR before capturing (UT-004)", () => {
    const out = mkdtempSync(join(tmpdir(), "brand-ut004-"));
    dirs.push(out);
    const target = join(REPO_ROOT, "packages/brand/ut004-shot.png");
    const result = run("shot", { BRAND_EXPORT_DIR: out }, ["http://127.0.0.1:9", target]);
    expect(result.status).toBe(1);
    expect(result.stderr).toContain("outside BRAND_EXPORT_DIR");
    expect(existsSync(target)).toBe(false);
  });

  it("resolves shot names under <export dir>/shots and refuses escapes (UT-004)", () => {
    const dir = realpathSync(mkdtempSync(join(tmpdir(), "brand-ut004-")));
    dirs.push(dir);
    expect(resolveExportFile(dir, "page.png", "shots")).toBe(join(dir, "shots", "page.png"));
    expect(resolveExportFile(dir, join(dir, "qa/page.png"), "shots")).toBe(
      join(dir, "qa", "page.png")
    );
    expect(() => resolveExportFile(dir, "../../escape.png", "shots")).toThrow(
      /outside BRAND_EXPORT_DIR/
    );
    expect(() => resolveExportFile(dir, join(REPO_ROOT, "x.png"), "shots")).toThrow(
      /outside BRAND_EXPORT_DIR/
    );
  });
});
