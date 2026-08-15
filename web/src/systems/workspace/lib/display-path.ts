import { normalizeRootDir } from "./project-workspaces";

/**
 * Contract a home-rooted absolute path to its `~` form for display. Clipboard,
 * tooltips, and API calls keep the absolute path — only the rendered string
 * contracts, matching the switcher set's micro-mono path treatment.
 */
export function contractHomePath(path: string, userHomeDir: string | undefined): string {
  if (!path || !userHomeDir) return path;
  const home = normalizeRootDir(userHomeDir);
  if (home.length === 0 || home === "/") return path;
  if (normalizeRootDir(path) === home) return "~";
  if (path.startsWith(`${home}/`)) return `~${path.slice(home.length)}`;
  return path;
}
