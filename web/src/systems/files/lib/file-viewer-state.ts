import type { FileContentResponse } from "../adapters/files-api";
import { filesErrorCode } from "../adapters/files-api";

/** Every truthful state the viewer can be in (`_uiux.md` S4); never a silent blank. */
export type FileViewerState =
  | { kind: "loading" }
  | { kind: "not_found" }
  | { kind: "outside" }
  | { kind: "denied" }
  | { kind: "not_a_file" }
  | { kind: "error"; message: string }
  | { kind: "text"; content: FileContentResponse; text: string }
  | { kind: "image"; content: FileContentResponse }
  | { kind: "image_too_large"; content: FileContentResponse }
  | { kind: "binary"; content: FileContentResponse }
  | { kind: "sensitive"; content: FileContentResponse };

/**
 * Errors that make retained content wrong, not merely old: after a refresh that reports one of
 * these, the viewer shows the failure instead of the last loaded text.
 */
const DEFINITIVE_ERRORS: ReadonlySet<string> = new Set([
  "file_not_found",
  "path_outside_root",
  "invalid_path",
  "path_denied",
  "not_a_file",
  "root_unavailable",
]);

/**
 * The viewer state for the latest read. TanStack Query keeps `data` after a failed refetch, so a
 * definitive failure (the file was deleted, the root is gone) replaces it; any other failed
 * refresh keeps the content and is reported as stale by `fileViewerStale`.
 */
export function fileViewerState(query: {
  data: FileContentResponse | undefined;
  error: Error | null;
}): FileViewerState {
  const definitive =
    query.error !== null && DEFINITIVE_ERRORS.has(filesErrorCode(query.error) ?? "");
  if (query.error && (!query.data || definitive)) {
    switch (filesErrorCode(query.error)) {
      case "file_not_found":
        return { kind: "not_found" };
      case "path_outside_root":
      case "invalid_path":
        return { kind: "outside" };
      case "path_denied":
        return { kind: "denied" };
      case "not_a_file":
        return { kind: "not_a_file" };
      default:
        return { kind: "error", message: query.error.message };
    }
  }
  const content = query.data;
  if (!content) return { kind: "loading" };
  switch (content.kind) {
    case "sensitive":
      return { kind: "sensitive", content };
    case "image":
      return content.image_base64 && !content.truncated
        ? { kind: "image", content }
        : { kind: "image_too_large", content };
    case "text":
      return { kind: "text", content, text: content.text ?? "" };
    default:
      return { kind: "binary", content };
  }
}

/** The content on screen is the last good read; the latest refresh failed transiently. */
export function fileViewerStale(query: {
  data: FileContentResponse | undefined;
  error: Error | null;
}): boolean {
  return (
    query.data !== undefined &&
    query.error !== null &&
    !DEFINITIVE_ERRORS.has(filesErrorCode(query.error) ?? "")
  );
}
