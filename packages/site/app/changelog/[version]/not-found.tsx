/** A version the changelog does not list, set like the site's 404 inside the changelog shell. */
import { cn } from "@compozy/ui";

import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, PrimaryCta, SecondaryCta, Small } from "@/components/site/primitives";
import { CHANGELOG_CUTOFF_TAG, COMPOZY_REPOSITORY } from "@/lib/changelog/types";

export default function ChangelogReleaseNotFound() {
  return (
    <section className={cn(CONTAINER, "flex flex-1 flex-col justify-center pt-40 pb-32 md:pt-48")}>
      <Small className="font-mono text-brand-strong">Release not found</Small>
      <div className="mt-5 max-w-[760px]">
        <H1>This version is not in the public changelog.</H1>
        <Lead className="max-w-[560px]">
          The site lists published GitHub releases from{" "}
          <span className="whitespace-nowrap">{CHANGELOG_CUTOFF_TAG}</span> onward.
        </Lead>
      </div>
      <div className="mt-10 flex flex-wrap gap-3">
        <PrimaryCta href="/changelog">Browse all releases</PrimaryCta>
        <SecondaryCta href={`${COMPOZY_REPOSITORY.url}/releases`}>GitHub Releases</SecondaryCta>
      </div>
    </section>
  );
}
