import { connection } from "next/server";
import { Suspense } from "react";

import { ChangelogList, ChangelogListFromUrl } from "@/components/changelog/changelog-list";
import { ChangelogUnavailable } from "@/components/changelog/changelog-unavailable";
import { loadChangelogReleases } from "@/lib/changelog/github-client";
import { toReleaseView, toTimelineRelease } from "@/lib/changelog/timeline";
import { changelogMetadata } from "./metadata";

export const metadata = changelogMetadata;
export const revalidate = 300;

export default async function ChangelogPage() {
  await connection();
  const result = await loadChangelogReleases();
  if (result.status === "unavailable") return <ChangelogUnavailable />;
  const releases = result.releases.map(release => toTimelineRelease(toReleaseView(release)));
  return (
    // The page renders per request, so the URL's view is known on the server and the fallback only
    // stands in when that is not the case.
    <Suspense fallback={<ChangelogList releases={releases} />}>
      <ChangelogListFromUrl releases={releases} />
    </Suspense>
  );
}
