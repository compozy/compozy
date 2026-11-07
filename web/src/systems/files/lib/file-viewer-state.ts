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

export function fileViewerState(query: {
  data: FileContentResponse | undefined;
  error: Error | null;
}): FileViewerState {
  if (query.error && !query.data) {
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
