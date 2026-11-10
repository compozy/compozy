"use client";

/** The root render failure, set like the site's 404: the marketing shell, a label, a headline. */
import { Button, cn } from "@compozy/ui";

import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, PrimaryCta, Small } from "@/components/site/primitives";
import { SiteShell } from "@/components/site/site-shell";

export default function Error({
  error,
  retry,
}: {
  error: Error & { digest?: string };
  retry: () => void;
}) {
  const detail = error.digest ? `Digest ${error.digest}` : "Runtime boundary failure";

  return (
    <SiteShell>
      <section
        className={cn(CONTAINER, "flex flex-1 flex-col justify-center pt-40 pb-32 md:pt-48")}
      >
        <Small className="font-mono text-destructive">Render failure</Small>
        <div className="mt-5 max-w-[760px]">
          <H1>The site hit a recoverable boundary.</H1>
          <Lead className="max-w-[560px]">
            The page failed while rendering. Retry the boundary; if it repeats, capture the detail
            below with the current route.
          </Lead>
        </div>
        <Small className="mt-6 font-mono">{detail}</Small>
        <div className="mt-10 flex flex-wrap gap-3">
          <PrimaryCta href="/">Back to home</PrimaryCta>
          <Button
            variant="outline"
            size="cta-lg"
            onClick={retry}
            className="h-[52px] border-site-light-150 bg-site-light-030 px-7 text-[15.5px]"
          >
            Retry boundary
          </Button>
        </div>
      </section>
    </SiteShell>
  );
}
