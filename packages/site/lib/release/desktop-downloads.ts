import { loadChangelogReleases } from "../changelog/github-client";
import { compareReleaseTags } from "../changelog/release-markdown";
import { COMPOZY_REPOSITORY, type ChangelogRelease } from "../changelog/types";

/** Always resolves to the newest stable GitHub release, so it never goes stale. */
export const LATEST_RELEASE_URL = `${COMPOZY_REPOSITORY.url}/releases/latest`;

export type DesktopTargetId = "mac-arm64" | "mac-x64" | "linux-appimage" | "linux-deb";

export interface DesktopTarget {
  id: DesktopTargetId;
  os: "macOS" | "Linux";
  arch: string;
  format: string;
  /** Asset name suffix after `CompozyOS-<version>` (electron-builder artifact names). */
  suffix: string;
}

export const DESKTOP_TARGETS: readonly DesktopTarget[] = [
  { id: "mac-arm64", os: "macOS", arch: "Apple silicon", format: "DMG", suffix: "-mac-arm64.dmg" },
  { id: "mac-x64", os: "macOS", arch: "Intel", format: "DMG", suffix: "-mac-x64.dmg" },
  {
    id: "linux-appimage",
    os: "Linux",
    arch: "x64",
    format: "AppImage",
    suffix: "-linux-x64.AppImage",
  },
  { id: "linux-deb", os: "Linux", arch: "x64", format: "deb", suffix: "-linux-x64.deb" },
];

const WINDOWS_COMMAND_ASSET = "compozy_windows_x86_64.zip";
const CHECKSUMS_ASSET = "checksums.txt";
const SIGNATURE_ASSET = "checksums.txt.sigstore.json";
const STABLE_TAG_PATTERN = /^v\d+\.\d+\.\d+$/;

export interface DesktopDownload {
  target: DesktopTarget;
  fileName: string;
  url: string;
  size: number;
}

export interface ReleaseFile {
  fileName: string;
  url: string;
  size: number;
}

/** One command-line package of a release (GoReleaser artifact names). */
export interface CommandTarget {
  os: "macOS" | "Linux";
  arch: string;
  format: string;
  /** Asset name; `{version}` stands for the bare release version (plain data, so it reaches client components). */
  pattern: string;
}

export const COMMAND_TARGETS: readonly CommandTarget[] = [
  {
    os: "macOS",
    arch: "Apple silicon",
    format: "tar.gz",
    pattern: "compozy_darwin_arm64.tar.gz",
  },
  { os: "macOS", arch: "Intel", format: "tar.gz", pattern: "compozy_darwin_x86_64.tar.gz" },
  { os: "Linux", arch: "x64", format: "tar.gz", pattern: "compozy_linux_x86_64.tar.gz" },
  { os: "Linux", arch: "ARM64", format: "tar.gz", pattern: "compozy_linux_arm64.tar.gz" },
  { os: "Linux", arch: "x64", format: "deb", pattern: "compozy_{version}_amd64.deb" },
  {
    os: "Linux",
    arch: "ARM64",
    format: "deb",
    pattern: "compozy_{version}_arm64.deb",
  },
  {
    os: "Linux",
    arch: "x64",
    format: "rpm",
    pattern: "compozy-{version}-1.x86_64.rpm",
  },
  {
    os: "Linux",
    arch: "ARM64",
    format: "rpm",
    pattern: "compozy-{version}-1.aarch64.rpm",
  },
];

export interface CommandPackage {
  target: CommandTarget;
  file: ReleaseFile;
}

export type DesktopRelease =
  | {
      status: "ready";
      tag: string;
      releaseUrl: string;
      publishedAt: string;
      downloads: DesktopDownload[];
      checksums: ReleaseFile | null;
      signature: ReleaseFile | null;
      windowsCommand: ReleaseFile | null;
      commandPackages: CommandPackage[];
    }
  | { status: "fallback"; releaseUrl: string };

export function desktopAssetName(tag: string, target: DesktopTarget): string {
  return `CompozyOS-${tag.replace(/^v/, "")}${target.suffix}`;
}

function releaseFile(release: ChangelogRelease, fileName: string): ReleaseFile | null {
  const asset = release.assets.find(item => item.name === fileName);
  return asset ? { fileName: asset.name, url: asset.url, size: asset.size } : null;
}

function commandPackages(release: ChangelogRelease): CommandPackage[] {
  const version = release.version.replace(/^v/, "");
  return COMMAND_TARGETS.flatMap(target => {
    const file = releaseFile(release, target.pattern.replace("{version}", version));
    return file ? [{ target, file }] : [];
  });
}

function desktopDownloads(release: ChangelogRelease): DesktopDownload[] {
  return DESKTOP_TARGETS.flatMap(target => {
    const fileName = desktopAssetName(release.version, target);
    const asset = release.assets.find(item => item.name === fileName);
    return asset ? [{ target, fileName, url: asset.url, size: asset.size }] : [];
  });
}

/**
 * Picks the newest stable (non-prerelease) release that actually publishes
 * desktop packages, and links each package by its real GitHub asset URL.
 */
export function selectDesktopRelease(releases: readonly ChangelogRelease[]): DesktopRelease {
  const stable = releases
    .filter(release => !release.prerelease && STABLE_TAG_PATTERN.test(release.version))
    .sort((a, b) => compareReleaseTags(b.version, a.version) ?? 0);

  for (const release of stable) {
    const downloads = desktopDownloads(release);
    if (downloads.length === 0) continue;
    return {
      status: "ready",
      tag: release.version,
      releaseUrl: release.url,
      publishedAt: release.publishedAt,
      downloads,
      checksums: releaseFile(release, CHECKSUMS_ASSET),
      signature: releaseFile(release, SIGNATURE_ASSET),
      windowsCommand: releaseFile(release, WINDOWS_COMMAND_ASSET),
      commandPackages: commandPackages(release),
    };
  }
  return { status: "fallback", releaseUrl: LATEST_RELEASE_URL };
}

/**
 * Resolves desktop downloads from the GitHub Releases API (cached with the
 * changelog's revalidation window). When GitHub is unavailable, every link
 * falls back to the always-current latest release page.
 */
export async function getDesktopRelease(): Promise<DesktopRelease> {
  const result = await loadChangelogReleases();
  if (result.status !== "ready") return { status: "fallback", releaseUrl: LATEST_RELEASE_URL };
  return selectDesktopRelease(result.releases);
}

/** Visitor platform as far as the browser can tell. `mac` means the CPU is unknown. */
export type VisitorPlatform =
  | "mac-arm64"
  | "mac-x64"
  | "mac"
  | "linux"
  | "windows"
  | "mobile"
  | "unknown";

export interface PlatformSignals {
  userAgent: string;
  /** `navigator.maxTouchPoints`; iPadOS reports a desktop Mac user agent. */
  maxTouchPoints?: number;
  /** `navigator.userAgentData` high-entropy `architecture` (Chromium only). */
  architecture?: string;
  /** Unmasked WebGL renderer string, when the browser exposes it. */
  gpuRenderer?: string;
}

function macArchitecture({ architecture, gpuRenderer }: PlatformSignals): VisitorPlatform {
  if (architecture === "arm") return "mac-arm64";
  if (architecture === "x86") return "mac-x64";
  const renderer = gpuRenderer ?? "";
  // Safari reports a generic "Apple GPU" on every Mac, so only a named chip is proof.
  if (/Apple M\d/.test(renderer)) return "mac-arm64";
  if (/Intel|AMD|Radeon|NVIDIA/i.test(renderer)) return "mac-x64";
  return "mac";
}

export function detectVisitorPlatform(signals: PlatformSignals): VisitorPlatform {
  const ua = signals.userAgent;
  if (/iPhone|iPad|iPod|Android/i.test(ua)) return "mobile";
  if (/Macintosh|Mac OS X/.test(ua)) {
    if ((signals.maxTouchPoints ?? 0) > 1) return "mobile";
    return macArchitecture(signals);
  }
  if (/Windows/.test(ua)) return "windows";
  if (/Linux|X11/.test(ua) && !/CrOS/.test(ua)) return "linux";
  return "unknown";
}
