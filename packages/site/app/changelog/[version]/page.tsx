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
import {
  loadChangelogReleaseByTag,
  loadChangelogReleases,
  loadReleasePullRequests,
} from "@/lib/changelog/github-client";
import { compareReleaseTags } from "@/lib/changelog/release-markdown";
import {
  COMPOZY_REPOSITORY,
  type ChangelogRelease,
  type ChangelogReleaseLookup,
} from "@/lib/changelog/types";
import { createPageMetadata } from "@/lib/site-config";

export const revalidate = 300;

type PageProps = { params: Promise<{ version: string }> };

function findPredecessor(
  releases: readonly ChangelogRelease[],
  currentVersion: string
): ChangelogRelease | undefined {
  return releases.reduce<ChangelogRelease | undefined>((predecessor, candidate) => {
    const candidateToCurrent = compareReleaseTags(candidate.version, currentVersion);
    if (candidateToCurrent === null || candidateToCurrent >= 0) return predecessor;
    if (!predecessor) return candidate;

    const candidateToPredecessor = compareReleaseTags(candidate.version, predecessor.version);
    return candidateToPredecessor !== null && candidateToPredecessor > 0 ? candidate : predecessor;
  }, undefined);
}

async function resolveRelease(
  releases: readonly ChangelogRelease[],
  version: string
): Promise<ChangelogReleaseLookup> {
  const cachedRelease = releases.find(release => release.version === version);
  if (cachedRelease) return { status: "found", release: cachedRelease, releases: [...releases] };
  return loadChangelogReleaseByTag(version);
}

function throwUnavailable(error: { message: string }): never {
  throw new Error(`GitHub changelog is unavailable: ${error.message}`);
}

export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const [{ version }, result] = await Promise.all([params, loadChangelogReleases()]);
  const lookup = await resolveRelease(result.status === "ready" ? result.releases : [], version);
  if (lookup.status === "unavailable") throwUnavailable(lookup.error);
  const release = lookup.status === "found" ? lookup.release : undefined;
  return createPageMetadata({
    title: release ? `${release.version} Changelog` : "Release not found",
    description: release?.summary ?? "Published CompozyOS release notes.",
    path: `/changelog/${encodeURIComponent(version)}`,
  });
}

export default async function ChangelogReleasePage({ params }: PageProps) {
  const [{ version }, result] = await Promise.all([params, loadChangelogReleases()]);
  const lookup = await resolveRelease(result.status === "ready" ? result.releases : [], version);
  if (lookup.status === "unavailable") throwUnavailable(lookup.error);
  if (lookup.status === "missing") notFound();
  const release = lookup.release;
  const predecessor = findPredecessor(lookup.releases, release.version);
  const comparison = predecessor
    ? {
        predecessorVersion: predecessor.version,
        url: `${COMPOZY_REPOSITORY.url}/compare/${encodeURIComponent(predecessor.version)}...${encodeURIComponent(release.version)}`,
      }
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
            {comparison && (
              <Button
                nativeButton={false}
                variant="outline"
                render={
                  <a
                    href={comparison.url}
                    target="_blank"
                    rel="noreferrer noopener"
                    aria-label={`Compare ${release.version} with ${comparison.predecessorVersion} on GitHub`}
                  />
                }
              >
                Compare with {comparison.predecessorVersion}
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
