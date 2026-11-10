import { SiteShell } from "@/components/site/site-shell";
import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, PrimaryCta, SecondaryCta, Small } from "@/components/site/primitives";
import { cn } from "@compozy/ui";

export default function NotFound() {
  return (
    <SiteShell>
      <section
        className={cn(CONTAINER, "flex flex-1 flex-col justify-center pt-40 pb-32 md:pt-48")}
      >
        <Small className="font-mono text-brand-strong">404</Small>
        <div className="mt-5 max-w-[760px]">
          <H1>This page is not here.</H1>
          <Lead className="max-w-[560px]">
            The link may be old, or the page moved. Start again from the home page or the docs.
          </Lead>
        </div>
        <div className="mt-10 flex flex-wrap gap-3">
          <PrimaryCta href="/">Back to home</PrimaryCta>
          <SecondaryCta href="/docs">Read the docs</SecondaryCta>
        </div>
      </section>
    </SiteShell>
  );
}
