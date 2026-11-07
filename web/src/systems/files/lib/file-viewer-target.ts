/**
 * What the explorer's viewer shows. `nonce` changes on every open request, so reopening the
 * same path jumps to its line again and brings back a sensitive file's gate (US-004.AC-4).
 */
export type FileViewerTarget =
  | { kind: "file"; path: string; line?: number; nonce: number }
  /** A transcript path outside the root, kept exactly as written (US-004.AC-5). */
  | { kind: "outside"; path: string; nonce: number };

let lastNonce = 0;

export function nextFileViewerNonce(): number {
  lastNonce += 1;
  return lastNonce;
}

export function fileViewerTarget(path: string, line?: number): FileViewerTarget {
  return {
    kind: "file",
    path,
    ...(line !== undefined ? { line } : {}),
    nonce: nextFileViewerNonce(),
  };
}

export function outsideViewerTarget(path: string): FileViewerTarget {
  return { kind: "outside", path, nonce: nextFileViewerNonce() };
}
