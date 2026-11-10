import type { Metadata } from "next";
import { notFound } from "next/navigation";

import { ReleaseDetail, type ReleaseRecord } from "@/components/changelog/release-detail";
import {
  loadChangelogReleaseByTag,
  loadChangelogReleases,
  loadReleasePullRequests,
} from "@/lib/changelog/github-client";
import { compareReleaseTags } from "@/lib/changelog/release-markdown";
import { toReleaseView } from "@/lib/changelog/timeline";
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
  const view = toReleaseView(release);
  const predecessor = findPredecessor(lookup.releases, release.version);
  const evidence = await loadReleasePullRequests(release.pullRequestNumbers);
  const named = new Set(
    Object.values(view.commits).flatMap(lines => lines.flatMap(line => line.prs))
  );
  const record: ReleaseRecord = {
    assets: release.assets,
    sourceArchiveUrl: release.sourceArchiveUrl,
    sourceZipUrl: release.sourceZipUrl,
    contributors: evidence.status === "ready" ? evidence.contributors : [],
    otherPullRequests:
      evidence.status === "ready"
        ? evidence.pullRequests.filter(pullRequest => !named.has(String(pullRequest.number)))
        : [],
    comparison: predecessor
      ? {
          version: predecessor.version,
          url: `${COMPOZY_REPOSITORY.url}/compare/${encodeURIComponent(predecessor.version)}...${encodeURIComponent(release.version)}`,
        }
      : null,
  };
  const line = lookup.releases.flatMap(item =>
    item.prerelease === release.prerelease ? [{ version: item.version, pre: item.prerelease }] : []
  );

  return <ReleaseDetail release={view} line={line} record={record} />;
}
