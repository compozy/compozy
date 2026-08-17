import { parse, stringify } from "yaml";
import { basename } from "node:path";

export interface UpdateFile {
  readonly sha512: string;
  readonly size: number;
  readonly url: string;
}

export interface UpdateManifest {
  readonly files: readonly UpdateFile[];
  readonly releaseDate: string;
  readonly version: string;
}

export function parseUpdateManifest(contents: string): UpdateManifest {
  const value: unknown = parse(contents);
  if (!value || typeof value !== "object") throw new Error("Update manifest must be an object.");
  const version = Reflect.get(value, "version");
  const releaseDate = Reflect.get(value, "releaseDate");
  const files = Reflect.get(value, "files");
  if (typeof version !== "string" || typeof releaseDate !== "string" || !Array.isArray(files)) {
    throw new Error("Update manifest is missing version, releaseDate, or files.");
  }
  const validated = files.map((file): UpdateFile => {
    if (!file || typeof file !== "object") throw new Error("Update manifest file is invalid.");
    const url = Reflect.get(file, "url");
    const sha512 = Reflect.get(file, "sha512");
    const size = Reflect.get(file, "size");
    if (typeof url !== "string" || typeof sha512 !== "string" || typeof size !== "number") {
      throw new Error("Update manifest file is incomplete.");
    }
    return { url, sha512, size };
  });
  return { version, releaseDate, files: validated };
}

export function rewriteUpdateManifestURLs(contents: string, downloadBase: string): string {
  const manifest = parseUpdateManifest(contents);
  const base = downloadBase.replace(/\/+$/u, "");
  if (!URL.canParse(`${base}/asset`))
    throw new Error("Release download base must be an absolute URL.");
  return stringify({
    ...manifest,
    files: manifest.files.map(file => ({
      ...file,
      url: `${base}/${basename(new URL(file.url, "https://manifest.invalid/").pathname)}`,
    })),
  });
}

export function mergeMacUpdateManifests(arm64: string, x64: string): string {
  const first = parseUpdateManifest(arm64);
  const second = parseUpdateManifest(x64);
  if (first.version !== second.version) {
    throw new Error("macOS update manifests must describe the same version.");
  }
  const files = new Map<string, UpdateFile>();
  for (const file of [...first.files, ...second.files]) {
    const existing = files.get(file.url);
    if (existing && (existing.sha512 !== file.sha512 || existing.size !== file.size)) {
      throw new Error(`Conflicting macOS update manifest entry: ${file.url}`);
    }
    files.set(file.url, file);
  }
  const ordered = [...files.values()].sort((left, right) => left.url.localeCompare(right.url));
  return stringify({
    version: first.version,
    files: ordered,
    releaseDate: first.releaseDate > second.releaseDate ? first.releaseDate : second.releaseDate,
  });
}
