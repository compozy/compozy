import { Eyebrow } from "@compozy/ui";
import { ArrowUpRight, Download } from "lucide-react";
import Link from "next/link";
import { formatBytes } from "@/components/blog/format";
import { SectionFrame } from "@/components/landing/primitives/section-frame";
import { SectionHeader } from "@/components/landing/primitives/section-header";
import {
  DESKTOP_TARGETS,
  type DesktopRelease,
  type DesktopTarget,
} from "@/lib/release/desktop-downloads";

const LINK_CLASS =
  "inline-flex items-center gap-1.5 text-sm text-fg underline decoration-line-strong underline-offset-4 transition-colors hover:text-brand-strong hover:decoration-brand";

interface Row {
  target: DesktopTarget;
  fileName: string;
  href: string;
  size: number | null;
}

function rowsFor(release: DesktopRelease, os: DesktopTarget["os"]): Row[] {
  const targets = DESKTOP_TARGETS.filter(target => target.os === os);
  if (release.status === "fallback") {
    return targets.map(target => ({
      target,
      fileName: `CompozyOS-<version>${target.suffix}`,
      href: release.releaseUrl,
      size: null,
    }));
  }
  return release.downloads.flatMap(download =>
    download.target.os === os
      ? [
          {
            target: download.target,
            fileName: download.fileName,
            href: download.url,
            size: download.size,
          },
        ]
      : []
  );
}

function PlatformCard({ os, note, rows }: { os: DesktopTarget["os"]; note: string; rows: Row[] }) {
  return (
    <article className="flex flex-col rounded-diagram border border-line bg-canvas">
      <header className="border-b border-line px-5 py-4">
        <h3 className="text-lg font-medium text-fg">{os}</h3>
        <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{note}</p>
      </header>
      <ul className="flex flex-1 flex-col divide-y divide-line">
        {rows.map(row => (
          <li key={row.target.id}>
            <a
              href={row.href}
              className="group flex items-center gap-4 px-5 py-4 transition-colors hover:bg-hover"
            >
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium text-fg">
                  {row.target.arch} · {row.target.format}
                </p>
                <p className="mt-1 truncate font-mono text-meta text-subtle">{row.fileName}</p>
              </div>
              {row.size !== null ? (
                <span className="shrink-0 font-mono text-meta text-subtle">
                  {formatBytes(row.size)}
                </span>
              ) : null}
              <Download
                aria-hidden
                className="size-4 shrink-0 text-subtle transition-colors group-hover:text-brand"
              />
            </a>
          </li>
        ))}
      </ul>
    </article>
  );
}

function WindowsCard({ release }: { release: DesktopRelease }) {
  const windowsCommand = release.status === "ready" ? release.windowsCommand : null;
  return (
    <article className="flex flex-col rounded-diagram border border-dashed border-line bg-canvas">
      <header className="border-b border-line px-5 py-4">
        <h3 className="text-lg font-medium text-fg">Windows</h3>
        <p className="mt-1 text-sm leading-relaxed text-muted-foreground">
          There is no desktop app for Windows yet.
        </p>
      </header>
      <div className="flex flex-1 flex-col gap-3 px-5 py-4 text-sm leading-relaxed text-muted-foreground">
        <p>
          Each release includes the <code className="font-mono text-fg">compozy</code> command for
          Windows x64 as a ZIP archive. Installing it is a manual step for now.
        </p>
        <a href={windowsCommand?.url ?? release.releaseUrl} className={LINK_CLASS}>
          {windowsCommand?.fileName ?? "Find it on the latest release"}
          <ArrowUpRight aria-hidden className="size-3.5" />
        </a>
      </div>
    </article>
  );
}

export function DownloadCatalog({ release }: { release: DesktopRelease }) {
  const versionLabel = release.status === "ready" ? release.tag : "the latest release";
  return (
    <SectionFrame
      id="all-downloads"
      background="surface"
      padY="md"
      className="border-b border-line"
    >
      <SectionHeader
        eyebrow="All downloads"
        title="Every desktop package."
        description={`Packages for ${versionLabel}, straight from GitHub Releases. The app checks for updates to itself and to the runtime it manages.`}
      />

      <div className="mt-10 grid gap-4 lg:grid-cols-3">
        <PlatformCard
          os="macOS"
          note="Notarized. Open the DMG and drag CompozyOS to Applications."
          rows={rowsFor(release, "macOS")}
        />
        <PlatformCard
          os="Linux"
          note="The AppImage updates itself in place. To update a .deb install, install the newer .deb over it."
          rows={rowsFor(release, "Linux")}
        />
        <WindowsCard release={release} />
      </div>

      <div className="mt-6 flex flex-col gap-3 rounded-diagram border border-line bg-canvas px-5 py-4 md:flex-row md:items-center md:gap-6">
        <Eyebrow className="text-subtle">Verify and read</Eyebrow>
        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
          {release.status === "ready" && release.checksums ? (
            <a href={release.checksums.url} className={LINK_CLASS}>
              {release.checksums.fileName}
            </a>
          ) : null}
          {release.status === "ready" ? (
            <Link href={`/changelog/${encodeURIComponent(release.tag)}`} className={LINK_CLASS}>
              Release notes for {release.tag}
            </Link>
          ) : (
            <Link href="/changelog" className={LINK_CLASS}>
              Release notes
            </Link>
          )}
          <a href={release.releaseUrl} className={LINK_CLASS}>
            Release on GitHub
            <ArrowUpRight aria-hidden className="size-3.5" />
          </a>
        </div>
      </div>
    </SectionFrame>
  );
}
