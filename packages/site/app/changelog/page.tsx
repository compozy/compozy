import { connection } from "next/server";

import { ChangelogList } from "@/components/changelog/changelog-list";
import { ChangelogUnavailable } from "@/components/changelog/changelog-unavailable";
import { loadChangelogReleases } from "@/lib/changelog/github-client";
import { betaBranch, toReleaseView, toTimelineRelease } from "@/lib/changelog/timeline";
import { changelogMetadata } from "./metadata";

export const metadata = changelogMetadata;
export const revalidate = 300;

export default async function ChangelogPage() {
  await connection();
  const result = await loadChangelogReleases();
  if (result.status === "unavailable") return <ChangelogUnavailable />;
  const releases = result.releases.map(release => toTimelineRelease(toReleaseView(release)));
  return <ChangelogList releases={releases} betas={betaBranch(releases)} />;
}
