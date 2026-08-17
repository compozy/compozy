export const frameworkSymlinks = [
  "Contents/Frameworks/Electron Framework.framework/Electron Framework",
  "Contents/Frameworks/Electron Framework.framework/Helpers",
  "Contents/Frameworks/Electron Framework.framework/Libraries",
  "Contents/Frameworks/Electron Framework.framework/Resources",
  "Contents/Frameworks/Electron Framework.framework/Versions/Current",
] as const;

export function requiredMacZipSymlinks(appBundle: string): readonly string[] {
  if (!appBundle.endsWith(".app")) throw new Error("macOS update zip must contain one app bundle.");
  return frameworkSymlinks.map(entry => `${appBundle}/${entry}`);
}

export function parseZipInfoLongListing(listing: string): Map<string, string> {
  const entries = new Map<string, string>();
  for (const line of listing.split(/\r?\n/u)) {
    const match = line.match(/^([dl-][rwx-]{9})(?:\s+\S+){8}\s+(.+)$/u);
    if (match?.[1] && match[2]) entries.set(match[2], match[1]);
  }
  return entries;
}

export function assertMacZipSymlinks(
  appBundle: string,
  entries: ReadonlyMap<string, string>
): void {
  for (const required of requiredMacZipSymlinks(appBundle)) {
    const mode = entries.get(required);
    if (!mode?.startsWith("l")) {
      throw new Error(`macOS update zip lost required symlink: ${required}`);
    }
  }
}
