import { posix, win32 } from "node:path";

/**
 * Credential locations relative to home. A target under any of them always confirms, even
 * inside a root: `user_scope.work_dir` is agent-mutable, so a root alone must never make
 * credentials one click away.
 */
const SENSITIVE_HOME_ENTRIES = [
  ".ssh",
  ".gnupg",
  ".aws",
  ".kube",
  ".docker",
  ".config/gh",
  ".netrc",
] as const;

/** Lexical, filesystem-free path rules for one platform. */
export class FileOpenPolicy {
  readonly #path: typeof posix;
  readonly #foldCase: boolean;

  constructor(platform: NodeJS.Platform) {
    this.#path = platform === "win32" ? win32 : posix;
    // Only win32 folds case. macOS volumes may be case-sensitive, so a lexical case mismatch
    // stays outside (and confirms); on a case-insensitive volume `realpath` returns the
    // on-disk case for both the target and the roots, so the real-path check still matches.
    this.#foldCase = platform === "win32";
  }

  /** Expands a leading `~/` with `home` and normalizes `.`/`..` segments lexically. */
  expand(path: string, home: string): string {
    const expanded = /^~[\\/]/.test(path) ? this.#path.join(home, path.slice(2)) : path;
    return this.#path.resolve(expanded);
  }

  normalize(path: string): string {
    return this.#path.resolve(path);
  }

  /** True when `target` equals `root` or is a separator-aware descendant of it. */
  contains(root: string, target: string): boolean {
    const relative = this.#path.relative(this.#key(root), this.#key(target));
    if (relative === "") return true;
    // win32 returns an absolute path when the drives differ.
    if (this.#path.isAbsolute(relative)) return false;
    return relative !== ".." && !relative.startsWith(`..${this.#path.sep}`);
  }

  same(left: string, right: string): boolean {
    return this.#key(left) === this.#key(right);
  }

  /** A root that is the filesystem root, a home directory, or an ancestor of one never counts. */
  discardsRoot(root: string, homes: readonly string[]): boolean {
    if (this.same(root, this.#path.parse(root).root)) return true;
    return homes.some(home => this.contains(root, home));
  }

  /** True when `target` is under a fixed credential location of any of `homes`. */
  isSensitive(target: string, homes: readonly string[]): boolean {
    return homes.some(home =>
      SENSITIVE_HOME_ENTRIES.some(entry => this.contains(this.#path.join(home, entry), target))
    );
  }

  #key(path: string): string {
    const normalized = this.#path.resolve(path);
    return this.#foldCase ? normalized.toLowerCase() : normalized;
  }
}
