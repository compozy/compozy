/** What the download page offers a visitor: one primary file per platform/arch, and every package grouped. */
import {
  COMMAND_TARGETS,
  DESKTOP_TARGETS,
  type DesktopRelease,
  type DesktopTargetId,
  type VisitorPlatform,
} from "./desktop-downloads";

export type Platform = "macos" | "linux" | "windows";
export type MacArch = "arm64" | "x64";
export type HeroMode = "ready" | "mobile" | "unavailable";

export interface Choice {
  platform: Platform;
  arch: MacArch;
}

/** The platform switch starts where detection points; an unknown Mac or OS gets the Apple silicon build. */
export function choiceFor(visitor: VisitorPlatform): Choice {
  switch (visitor) {
    case "linux":
      return { platform: "linux", arch: "arm64" };
    case "windows":
      return { platform: "windows", arch: "arm64" };
    case "mac-x64":
      return { platform: "macos", arch: "x64" };
    case "mac-arm64":
    case "mac":
    case "mobile":
    case "unknown":
      return { platform: "macos", arch: "arm64" };
  }
}

/** A phone has nothing to install; otherwise the release decides. */
export function heroModeFor(release: DesktopRelease, visitor: VisitorPlatform): HeroMode {
  if (visitor === "mobile") return "mobile";
  return release.status === "ready" ? "ready" : "unavailable";
}

/** One downloadable file as the page lists it. `fileName`/`size` are null when the release is unknown. */
export interface DownloadItem {
  name: string;
  detail: string;
  fileName: string | null;
  href: string;
  size: number | null;
}

export interface Offer {
  cta: string;
  item: DownloadItem;
}

function desktopItem(release: DesktopRelease, id: DesktopTargetId): DownloadItem {
  const target = DESKTOP_TARGETS.find(item => item.id === id);
  if (!target) throw new Error(`unknown desktop target ${id}`);
  const download =
    release.status === "ready" ? release.downloads.find(item => item.target.id === id) : undefined;
  return {
    name: target.arch,
    detail: target.format,
    fileName: download?.fileName ?? null,
    href: download?.url ?? release.releaseUrl,
    size: download?.size ?? null,
  };
}

function windowsItem(release: DesktopRelease): DownloadItem {
  const file = release.status === "ready" ? release.windowsCommand : null;
  return {
    name: "x64",
    detail: "ZIP",
    fileName: file?.fileName ?? null,
    href: file?.url ?? release.releaseUrl,
    size: file?.size ?? null,
  };
}

const MAC_TARGET: Record<MacArch, DesktopTargetId> = { arm64: "mac-arm64", x64: "mac-x64" };

/** The primary button: the release's real file for this platform and architecture. */
export function offerFor(release: DesktopRelease, { platform, arch }: Choice): Offer {
  if (platform === "linux")
    return { cta: "Download for Linux", item: desktopItem(release, "linux-appimage") };
  if (platform === "windows") return { cta: "Download the command", item: windowsItem(release) };
  return { cta: "Download for macOS", item: desktopItem(release, MAC_TARGET[arch]) };
}

export function linuxDebItem(release: DesktopRelease): DownloadItem {
  return desktopItem(release, "linux-deb");
}

export interface DownloadGroup {
  label: string;
  note: string;
  items: DownloadItem[];
}

const DESKTOP_GROUPS: ReadonlyArray<readonly [string, readonly DesktopTargetId[]]> = [
  ["macOS", ["mac-arm64", "mac-x64"]],
  ["Linux", ["linux-appimage", "linux-deb"]],
];

/** Desktop apps first, then the Windows command. When the release is unavailable every row opens it on GitHub. */
export function desktopGroups(release: DesktopRelease): DownloadGroup[] {
  return [
    ...DESKTOP_GROUPS.map(([label, ids]) => ({
      label,
      note: "Desktop app",
      items: ids.map(id => desktopItem(release, id)),
    })),
    { label: "Windows", note: "Command only", items: [windowsItem(release)] },
  ];
}

/** Command-line packages per OS; a ready release lists only the files it published. */
export function commandGroups(release: DesktopRelease): DownloadGroup[] {
  return (["macOS", "Linux"] as const).map(os => ({
    label: os,
    note: "Command only",
    items:
      release.status === "ready"
        ? release.commandPackages.flatMap(({ target, file }) =>
            target.os === os
              ? [
                  {
                    name: target.arch,
                    detail: target.format,
                    fileName: file.fileName,
                    href: file.url,
                    size: file.size,
                  },
                ]
              : []
          )
        : COMMAND_TARGETS.flatMap(target =>
            target.os === os
              ? [
                  {
                    name: target.arch,
                    detail: target.format,
                    fileName: null,
                    href: release.releaseUrl,
                    size: null,
                  },
                ]
              : []
          ),
  }));
}

export function megabytes(bytes: number) {
  return `${Math.round(bytes / 1048576)} MB`;
}
