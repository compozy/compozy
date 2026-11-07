/**
 * What the explorer's viewer shows. `nonce` changes on every open request, so reopening the
 * same path jumps to its line again and brings back a sensitive file's gate (US-004.AC-4).
 */
/** A find request carried by an open: Search pre-fills the bar (US-015.AC-4). */
export interface ViewerFindSeed {
  query: string;
  matchCase: boolean;
}

export type FileViewerTarget =
  /** `find` opens the Find in file bar pre-filled (an open from Search, US-015.AC-4). */
  | { kind: "file"; path: string; line?: number; find?: ViewerFindSeed; nonce: number }
  /** A transcript path outside the root, kept exactly as written (US-004.AC-5). */
  | { kind: "outside"; path: string; nonce: number };

let lastNonce = 0;

export function nextFileViewerNonce(): number {
  lastNonce += 1;
  return lastNonce;
}

export function fileViewerTarget(
  path: string,
  line?: number,
  find?: ViewerFindSeed
): FileViewerTarget {
  return {
    kind: "file",
    path,
    ...(line !== undefined ? { line } : {}),
    ...(find ? { find } : {}),
    nonce: nextFileViewerNonce(),
  };
}

export function outsideViewerTarget(path: string): FileViewerTarget {
  return { kind: "outside", path, nonce: nextFileViewerNonce() };
}
