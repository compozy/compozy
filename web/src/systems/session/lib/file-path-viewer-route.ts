import { fileViewerTarget, outsideViewerTarget, type FileViewerTarget } from "@/systems/files";

import {
  resolveRootRelative,
  type FilePathPlatformOptions,
  type FilePathRoot,
  type FilePathToken,
} from "./file-path-links";

/** The latest viewer request per session; an older click that resolves later is dropped. */
const latestRequest = new Map<string, number>();

export interface FilePathViewerRoute {
  sessionId: string;
  /** Reads the session root at click time; `null` when it cannot be known. */
  resolveRoot: () => Promise<FilePathRoot | null>;
  open: (target: FileViewerTarget) => void;
}

/**
 * Routes a transcript path to the session's viewer (spec Data flow 1–3): inside the root it
 * opens Files at the path and line, otherwise "Outside this project" with the path as written.
 * Two quick clicks open only the last one (US-005.EC-4).
 */
export async function openFilePathInViewer(
  token: FilePathToken,
  route: FilePathViewerRoute,
  options: FilePathPlatformOptions = {}
): Promise<void> {
  const request = (latestRequest.get(route.sessionId) ?? 0) + 1;
  latestRequest.set(route.sessionId, request);
  const root = await route.resolveRoot();
  if (latestRequest.get(route.sessionId) !== request) return;
  const resolved = root
    ? resolveRootRelative(token, root, options)
    : { inside: false as const, path: token.path };
  route.open(
    resolved.inside
      ? fileViewerTarget(resolved.path, resolved.line)
      : outsideViewerTarget(resolved.path)
  );
}
