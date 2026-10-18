import { CompozyConfigError } from "./errors.js";

/** Replaces Node discovery in browser bundles; this module never reads process or files. */
export async function discover(options: {
  baseUrl?: string;
  home?: string;
}): Promise<{ baseUrl: string; token?: string }> {
  const value = options.baseUrl ?? "http://localhost:2123";
  try {
    const url = new URL(value);
    if (
      !["http:", "https:"].includes(url.protocol) ||
      url.username ||
      url.password ||
      url.search ||
      url.hash
    ) {
      throw new Error("expected an HTTP(S) URL without credentials, query, or fragment");
    }
    return { baseUrl: url.href.replace(/\/$/, "") };
  } catch (cause) {
    throw new CompozyConfigError("Invalid baseUrl: expected an absolute HTTP(S) URL", { cause });
  }
}
