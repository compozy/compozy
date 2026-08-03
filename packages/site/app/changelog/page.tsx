import { Eyebrow } from "@compozy/ui";
import { Rss } from "lucide-react";
import Link from "next/link";
import { ChangelogTocRail } from "@/components/blog/changelog-toc-rail";
import { ReleaseEntry } from "@/components/blog/release-entry";
import { loadChangelogReleases } from "@/lib/changelog/github-client";
import { changelogMetadata } from "./metadata";

export const metadata = changelogMetadata;
export const revalidate = 300;

function UnavailableChangelog() {
  return (
    <section className="mt-12 rounded-xl border border-line bg-canvas-soft p-6">
      <Eyebrow className="text-warning">GitHub temporarily unavailable</Eyebrow>
      <h2 className="mt-4 font-sans text-site-empty-title font-semibold tracking-tight text-fg">
        The release archive could not be loaded.
      </h2>
      <p className="mt-4 max-w-[62ch] text-sm leading-7 text-muted">
        The changelog reads directly from published GitHub Releases. Try again shortly, or open the
        repository release archive for the canonical notes.
      </p>
      <a
        href="https://github.com/compozy/compozy/releases"
        target="_blank"
        rel="noreferrer noopener"
        className="mt-6 inline-flex text-sm font-medium text-accent"
      >
        Open GitHub Releases →
      </a>
    </section>
  );
}

export default async function ChangelogPage() {
  const result = await loadChangelogReleases();
  const releases = result.status === "ready" ? result.releases : [];

  return (
    <>
      <section className="border-b border-line px-4 pt-14 pb-12">
        <div className="mx-auto max-w-site-layout-width">
          <div className="flex items-center gap-3">
            <Eyebrow className="text-accent">CHANGELOG</Eyebrow>
            <span className="inline-block h-px w-9 bg-line" />
            <Eyebrow className="text-muted">Published GitHub releases</Eyebrow>
          </div>
          <h1 className="mt-6 max-w-[24ch] font-display text-site-page-title font-normal leading-none tracking-tight text-fg">
            Complete notes for every CompozyOS release.
          </h1>
          <p className="mt-5 max-w-[62ch] text-lg leading-7 text-muted">
            Features, fixes, breaking changes, pull requests, contributors, and verified release
            assets—sourced from the same GitHub release that ships the binary.
          </p>
          <Link
            href="/changelog/feed.xml"
            className="mt-7 inline-flex h-8 items-center gap-1.5 rounded-full border border-line px-3.5 font-sans text-small-body text-muted hover:text-fg"
          >
            <Rss size={12} aria-hidden />
            <Eyebrow className="text-muted">RSS</Eyebrow>
          </Link>
        </div>
      </section>

      <section className="px-4 pt-6 pb-20">
        <div className="mx-auto grid max-w-site-layout-width gap-12 lg:grid-cols-[minmax(0,1fr)_280px]">
          <div>
            {result.status === "unavailable" ? (
              <UnavailableChangelog />
            ) : releases.length === 0 ? (
              <section className="mt-12 rounded-xl border border-line bg-canvas-soft p-6">
                <Eyebrow className="text-muted">No published releases</Eyebrow>
                <h2 className="mt-4 font-sans text-site-empty-title font-semibold tracking-tight text-fg">
                  Releases from v0.3.0-beta.1 onward will appear here.
                </h2>
              </section>
            ) : (
              releases.map(release => <ReleaseEntry key={release.version} release={release} />)
            )}
          </div>
          {releases.length > 0 && <ChangelogTocRail releases={releases} />}
        </div>
      </section>
    </>
  );
}
