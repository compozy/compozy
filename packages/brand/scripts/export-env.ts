// Shared environment of the export scripts (_dx.md): where exports go and which dev server they
// capture. Exports never land inside the git working tree (ADR-003, US-003.AC-3).
import { existsSync, realpathSync } from "node:fs";
import { homedir } from "node:os";
import { dirname, isAbsolute, join, relative, resolve, sep } from "node:path";
import { fileURLToPath } from "node:url";

/** packages/brand → the repository working tree that exports must stay out of. */
export const REPO_ROOT = resolve(fileURLToPath(new URL("../../..", import.meta.url)));
export const DEFAULT_EXPORT_DIR = "~/Dev/compozy/_rebranding/exports";
export const DEFAULT_BASE_URL = "http://localhost:4620";

/** The real path of `p`, resolving symlinks through its nearest existing ancestor. */
function realish(p: string): string {
  let head = p;
  const tail: string[] = [];
  while (!existsSync(head) && dirname(head) !== head) {
    tail.unshift(head.slice(dirname(head).length + 1));
    head = dirname(head);
  }
  return join(realpathSync(head), ...tail);
}

/** Parent traversal is a whole `..` path component; `..exports` is an ordinary child name. */
const isInside = (child: string, parent: string) => {
  const rel = relative(parent, child);
  const escapes = rel === ".." || rel.startsWith(`..${sep}`) || isAbsolute(rel);
  return !escapes;
};

/** `BRAND_EXPORT_DIR` (default ~/Dev/compozy/_rebranding/exports), refused inside the repo. */
export function resolveExportDir(
  env: Record<string, string | undefined>,
  repoRoot: string = REPO_ROOT
): string {
  const raw = env.BRAND_EXPORT_DIR?.trim() || DEFAULT_EXPORT_DIR;
  const expanded = raw === "~" || raw.startsWith("~/") ? join(homedir(), raw.slice(1)) : raw;
  const dir = realish(resolve(expanded));
  if (isInside(dir, realish(repoRoot))) {
    throw new Error(
      `BRAND_EXPORT_DIR ${dir} is inside the git working tree ${repoRoot} — exports stay out of git (ADR-003); pick a directory outside it`
    );
  }
  return dir;
}

/** `BRAND_BASE_URL` without a trailing slash (default http://localhost:4620). */
export const baseUrlOf = (env: Record<string, string | undefined>) =>
  (env.BRAND_BASE_URL?.trim() || DEFAULT_BASE_URL).replace(/\/+$/, "");

export const unreachable = (base: string) =>
  `brand dev server not reachable at ${base} — run \`bun run dev\` first`;

/** GET `${base}${path}` as JSON; a refused connection becomes the _dx.md "not reachable" error. */
export async function fetchJson<T>(
  base: string,
  path: string,
  fetchImpl: typeof fetch = fetch
): Promise<T> {
  let res: Response;
  try {
    res = await fetchImpl(`${base}${path}`);
  } catch {
    throw new Error(unreachable(base));
  }
  if (!res.ok) throw new Error(`${base}${path} answered ${res.status}`);
  return (await res.json()) as T;
}

/** Throws the "not reachable" error unless the dev server answers at `base`. */
export async function assertReachable(base: string, fetchImpl: typeof fetch = fetch) {
  try {
    await fetchImpl(base, { method: "HEAD" });
  } catch {
    throw new Error(unreachable(base));
  }
}

export function fail(message: string): never {
  console.error(`error: ${message}`);
  process.exit(1);
}

export const errorText = (e: unknown) =>
  e instanceof Error ? e.message.split("\n")[0] : String(e);

/** Fonts loaded and every image decoded inside `selector` (runs in the page). */
export const settleInPage = async (selector: string) => {
  await document.fonts.ready;
  const root = document.querySelector(selector);
  const imgs = root ? Array.from(root.querySelectorAll("img")) : [];
  await Promise.all(
    imgs.map(async img => {
      if (!img.complete) {
        await new Promise(done => {
          img.addEventListener("load", done, { once: true });
          img.addEventListener("error", done, { once: true });
        });
      }
      await img.decode().catch(() => undefined);
    })
  );
};
