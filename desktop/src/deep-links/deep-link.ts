const RELEASE_SCHEME = "compozyos:";
const DEVELOPMENT_SCHEME = "compozyos-dev:";

export type DeepLinkTarget =
  | { readonly kind: "default" }
  | { readonly kind: "product"; readonly path: string };

function validProductPath(path: string): boolean {
  if (
    !path.startsWith("/") ||
    path.startsWith("//") ||
    path.includes("://") ||
    path.includes("\\")
  ) {
    return false;
  }
  return !path
    .split("/")
    .some(segment => segment === "." || segment === ".." || segment.includes("\0"));
}

export function parseDeepLink(raw: string): DeepLinkTarget {
  let link: URL;
  try {
    link = new URL(raw);
  } catch {
    return { kind: "default" };
  }
  if (
    (link.protocol !== RELEASE_SCHEME && link.protocol !== DEVELOPMENT_SCHEME) ||
    link.hostname !== "open" ||
    link.username !== "" ||
    link.password !== "" ||
    link.hash !== ""
  ) {
    return { kind: "default" };
  }
  const separator = raw.indexOf("://");
  if (separator < 0) return { kind: "default" };
  const rawTarget = raw
    .slice(separator + 3)
    .split("#", 1)[0]
    ?.split("?", 1)[0];
  const rawPath = rawTarget?.startsWith("open") ? rawTarget.slice("open".length) : null;
  if (rawPath === null) return { kind: "default" };
  let decoded: string;
  try {
    decoded = decodeURIComponent(rawPath);
  } catch {
    return { kind: "default" };
  }
  if (!validProductPath(decoded)) return { kind: "default" };
  return { kind: "product", path: `${decoded}${link.search}` };
}

export function lastDeepLink(arguments_: readonly string[]): string | null {
  return (
    arguments_.findLast(
      argument => argument.startsWith("compozyos://") || argument.startsWith("compozyos-dev://")
    ) ?? null
  );
}

export class DeepLinkQueue {
  #pending: string | null = null;
  #ready = false;

  push(raw: string): DeepLinkTarget {
    const target = parseDeepLink(raw);
    this.#pending = target.kind === "product" ? target.path : "/";
    return target;
  }

  setReady(): string | null {
    this.#ready = true;
    return this.take();
  }

  reset(): void {
    this.#ready = false;
  }

  take(): string | null {
    if (!this.#ready) return null;
    const pending = this.#pending;
    this.#pending = null;
    return pending;
  }
}
