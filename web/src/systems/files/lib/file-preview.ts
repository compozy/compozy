import type { FilesRootRef } from "../adapters/files-api";

/** Text formats the viewer can render besides their source. */
export type TextPreviewKind = "markdown" | "html" | "table" | "mermaid" | "notebook" | "svg";

/** Binary formats the viewer plays or draws from the daemon's raw-bytes route. */
export type MediaPreviewKind = "video" | "audio" | "pdf";

/** How a file with a rendered form is shown; remembered per `TextPreviewKind`. */
export type FileViewerMode = "source" | "preview" | "split";

const TEXT_PREVIEWS: Record<string, TextPreviewKind> = {
  md: "markdown",
  mdx: "markdown",
  markdown: "markdown",
  html: "html",
  htm: "html",
  csv: "table",
  tsv: "table",
  tab: "table",
  mmd: "mermaid",
  mermaid: "mermaid",
  ipynb: "notebook",
  svg: "svg",
};

const MEDIA_PREVIEWS: Record<string, MediaPreviewKind> = {
  mp4: "video",
  m4v: "video",
  webm: "video",
  mov: "video",
  ogv: "video",
  mp3: "audio",
  wav: "audio",
  ogg: "audio",
  oga: "audio",
  m4a: "audio",
  aac: "audio",
  flac: "audio",
  opus: "audio",
  pdf: "pdf",
};

function extension(path: string): string {
  const name = path.slice(path.lastIndexOf("/") + 1);
  const dot = name.lastIndexOf(".");
  return dot <= 0 ? "" : name.slice(dot + 1).toLowerCase();
}

// Protocol-relative `//host/path` counts as an https link; any other scheme stays blocked.
const EXTERNAL_LINK = /^(https?:|mailto:|\/\/)/i;

/** An http(s), mailto or protocol-relative link, which opens outside the viewer. */
export function isExternalLink(href: string): boolean {
  return EXTERNAL_LINK.test(href);
}

/** Opens an external link in the browser, protocol-relative ones over https. */
export function openExternalLink(href: string): void {
  const target = href.startsWith("//") ? `https:${href}` : href;
  window.open(target, "_blank", "noopener,noreferrer");
}

// Own keys only: `notes.constructor` must not find `Object.prototype.constructor`.
export function textPreviewKind(path: string): TextPreviewKind | null {
  const key = extension(path);
  return Object.hasOwn(TEXT_PREVIEWS, key) ? (TEXT_PREVIEWS[key] ?? null) : null;
}

export function mediaPreviewKind(path: string): MediaPreviewKind | null {
  const key = extension(path);
  return Object.hasOwn(MEDIA_PREVIEWS, key) ? (MEDIA_PREVIEWS[key] ?? null) : null;
}

function encodePath(path: string): string {
  return path
    .split("/")
    .filter(segment => segment.length > 0)
    .map(encodeURIComponent)
    .join("/");
}

function rootPrefix(root: FilesRootRef): string {
  const workspace = `/api/workspaces/${encodeURIComponent(root.workspaceId)}`;
  if (root.kind === "session") {
    return `${workspace}/sessions/${encodeURIComponent(root.sessionId)}`;
  }
  return root.worktreeId
    ? `${workspace}/worktrees/${encodeURIComponent(root.worktreeId)}`
    : workspace;
}

/**
 * Absolute URL of a root-relative file on the daemon's raw-bytes route. The root lives in the
 * path, so relative references inside a previewed document resolve within the same root; the
 * session scope query does not survive that resolution, so callers rewrite asset URLs through
 * `rawAssetUrl`.
 */
export function rawFileUrl(root: FilesRootRef, path: string, version?: number): string {
  const url = new URL(`${rootPrefix(root)}/files/raw/${encodePath(path)}`, window.location.origin);
  // Session reads look the session up across profiles, like every other session files read.
  if (root.kind === "session") url.searchParams.set("all_profiles", "true");
  // A new version is a new URL, so media, PDF and image elements request the bytes again.
  if (version) url.searchParams.set("v", String(version));
  return url.href;
}

/** The raw-bytes URL of the directory holding `path`, with a trailing slash (a `<base>`). */
export function rawDirectoryUrl(root: FilesRootRef, path: string): string {
  const slash = path.lastIndexOf("/");
  const directory = slash < 0 ? "" : path.slice(0, slash);
  const base = new URL(`${rootPrefix(root)}/files/raw/`, window.location.origin).href;
  return directory ? `${base}${encodePath(directory)}/` : base;
}

/**
 * The root-relative path a URL on this root's raw-bytes route names, or `null` for any other
 * URL (another root, another route, an external site).
 */
export function pathFromRawUrl(root: FilesRootRef, url: string): string | null {
  let parsed: URL;
  try {
    parsed = new URL(url, window.location.origin);
  } catch {
    return null;
  }
  if (parsed.origin !== window.location.origin) return null;
  const prefix = `${rootPrefix(root)}/files/raw/`;
  if (!parsed.pathname.startsWith(prefix)) return null;
  try {
    const path = decodeURIComponent(parsed.pathname.slice(prefix.length));
    return path.length > 0 ? path : null;
  } catch {
    return null;
  }
}

/**
 * Resolves a reference authored inside `fromPath` (a Markdown link or image) to a root-relative
 * path: `./b.md`, `../docs/a.md` and `/README.md` (root-anchored). The fragment and query are
 * dropped. Returns `null` for external URLs or a path that climbs out of the root.
 */
export function resolveRelativePath(fromPath: string, href: string): string | null {
  const trimmed = href.trim();
  if (trimmed === "" || trimmed.startsWith("#") || /^[a-z][a-z0-9+.-]*:/i.test(trimmed)) {
    return null;
  }
  if (trimmed.startsWith("//")) return null;
  const clean = trimmed.split(/[?#]/)[0] ?? "";
  let decoded: string;
  try {
    decoded = decodeURIComponent(clean);
  } catch {
    decoded = clean;
  }
  const slash = fromPath.lastIndexOf("/");
  const segments = decoded.startsWith("/")
    ? []
    : slash < 0
      ? []
      : fromPath.slice(0, slash).split("/");
  for (const segment of decoded.split("/")) {
    if (segment === "" || segment === ".") continue;
    if (segment === "..") {
      if (segments.length === 0) return null;
      segments.pop();
    } else {
      segments.push(segment);
    }
  }
  return segments.length > 0 ? segments.join("/") : null;
}

/** A URL resolved inside a previewed document, re-pointed at this root's raw route with its scope query. */
export function rawAssetUrl(root: FilesRootRef, url: string): string {
  const path = pathFromRawUrl(root, url);
  if (!path) return url;
  // `diagram.svg#detail` keeps the fragment that selects part of the asset.
  const hash = url.indexOf("#");
  return hash < 0 ? rawFileUrl(root, path) : `${rawFileUrl(root, path)}${url.slice(hash)}`;
}
