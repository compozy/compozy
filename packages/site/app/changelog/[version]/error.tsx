"use client";

/**
 * A release page that could not render. The page throws when GitHub is unavailable so a cached
 * page is never replaced by an error state; this says so in plain words and points to GitHub.
 */
import { Button, cn } from "@compozy/ui";

import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, PrimaryCta, SecondaryCta, Small } from "@/components/site/primitives";
import { COMPOZY_REPOSITORY } from "@/lib/changelog/types";

export default function ChangelogReleaseError({ retry }: { retry: () => void }) {
  return (
    <section className={cn(CONTAINER, "flex flex-1 flex-col justify-center pt-40 pb-32 md:pt-48")}>
      <Small className="font-mono text-warning">Changelog</Small>
      <div className="mt-5 max-w-[760px]">
        <H1>The release notes couldn't be loaded</H1>
        <Lead className="max-w-[560px]">
          They come from GitHub, which isn't responding right now. Try again in a moment, or read
          them on GitHub Releases.
        </Lead>
      </div>
      <div className="mt-10 flex flex-wrap gap-3">
        <PrimaryCta href={`${COMPOZY_REPOSITORY.url}/releases`}>GitHub Releases</PrimaryCta>
        <Button
          variant="outline"
          size="cta-lg"
          onClick={retry}
          className="h-[52px] border-site-light-150 bg-site-light-030 px-7 text-[15.5px]"
        >
          Try again
        </Button>
        <SecondaryCta href="/changelog">All releases</SecondaryCta>
      </div>
    </section>
  );
}
