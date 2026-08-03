import { Button, Eyebrow } from "@compozy/ui";
import { ArrowLeft, ArrowUpRight } from "lucide-react";
import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { DateStamp } from "@/components/blog/date-stamp";
import { ReleaseDetailRail } from "@/components/blog/release-detail-rail";
import { ReleaseEvidence } from "@/components/blog/release-evidence";
import { ReleaseMarkdown } from "@/components/blog/release-markdown";
import { MonoBadge } from "@/components/blog/mono-badge";
import { loadChangelogReleases, loadReleasePullRequests } from "@/lib/changelog/github-client";
import { COMPOZY_REPOSITORY } from "@/lib/changelog/types";
import { createPageMetadata } from "@/lib/site-config";

export const revalidate = 300;

type PageProps = { params: Promise<{ version: string }> };

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const [{ version }, result] = await Promise.all([params, loadChangelogReleases()]);
  const release =
    result.status === "ready" ? result.releases.find(item => item.version === version) : undefined;
  return createPageMetadata({
    title: release ? `${release.version} Changelog` : "Release not found",
    description: release?.summary ?? "Published CompozyOS release notes.",
    path: `/changelog/${encodeURIComponent(version)}`,
  });
}

export default async function ChangelogReleasePage({ params }: PageProps) {
  const [{ version }, result] = await Promise.all([params, loadChangelogReleases()]);
  if (result.status === "unavailable") {
    throw new Error(`GitHub changelog is unavailable: ${result.error.message}`);
  }
  const releaseIndex = result.releases.findIndex(item => item.version === version);
  if (releaseIndex < 0) notFound();
  const release = result.releases[releaseIndex];
  const predecessor = result.releases[releaseIndex + 1];
  const compareUrl = predecessor
    ? `${COMPOZY_REPOSITORY.url}/compare/${encodeURIComponent(predecessor.version)}...${encodeURIComponent(release.version)}`
    : null;
  const evidence = await loadReleasePullRequests(release.pullRequestNumbers);

  return (
    <main>
      <header className="border-b border-line px-4 pt-12 pb-12">
        <div className="mx-auto max-w-site-layout-width">
          <Link
            href="/changelog"
            className="inline-flex items-center gap-2 text-sm text-muted hover:text-fg"
          >
            <ArrowLeft className="size-3.5" aria-hidden /> All releases
          </Link>
          <div className="mt-8 flex flex-wrap items-center gap-3">
            <Eyebrow className="text-accent">CHANGELOG</Eyebrow>
            <span className="inline-block h-px w-9 bg-line" />
            <DateStamp date={release.publishedAt} tracking="wide" />
          </div>
          <h1 className="mt-5 font-mono text-site-page-title font-medium leading-none tracking-tight text-fg">
            {release.version}
          </h1>
          <p className="mt-5 max-w-[68ch] text-lg leading-8 text-muted">{release.summary}</p>
          <div className="mt-6 flex flex-wrap gap-2">
            <MonoBadge>{release.channel.toUpperCase()}</MonoBadge>
            {release.categories.map(category => (
              <MonoBadge key={category.id}>
                {category.label} · {category.itemCount}
              </MonoBadge>
            ))}
          </div>
          <div className="mt-8 flex flex-wrap gap-3">
            <Button
              nativeButton={false}
              render={
                <a
                  href={release.url}
                  target="_blank"
                  rel="noreferrer noopener"
                  aria-label={`View ${release.version} on GitHub`}
                />
              }
            >
              View release on GitHub <ArrowUpRight className="size-4" aria-hidden />
            </Button>
            {compareUrl && (
              <Button
                nativeButton={false}
                variant="outline"
                render={
                  <a
                    href={compareUrl}
                    target="_blank"
                    rel="noreferrer noopener"
                    aria-label={`Compare ${release.version} with ${predecessor.version} on GitHub`}
                  />
                }
              >
                Compare with {predecessor.version}
              </Button>
            )}
          </div>
        </div>
      </header>
      <section className="px-4 py-14">
        <div className="mx-auto grid max-w-site-layout-width gap-14 lg:grid-cols-[minmax(0,1fr)_280px]">
          <article className="min-w-0">
            <ReleaseMarkdown body={release.body} pullRequestNumbers={release.pullRequestNumbers} />
            <ReleaseEvidence release={release} evidence={evidence} />
          </article>
          <ReleaseDetailRail release={release} />
        </div>
      </section>
    </main>
  );
}
