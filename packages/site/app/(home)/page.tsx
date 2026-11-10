import {
  AlwaysOn,
  Automations,
  BuiltIn,
  Community,
  Comparison,
  Extensibility,
  FinalCta,
  Hero,
  Install,
  Loops,
  Orchestrate,
  Problem,
  Providers,
  Supervise,
  UseCases,
} from "@/components/landing";
import { WebSiteJsonLd } from "@/components/seo/structured-data";
import { loadChangelogReleases } from "@/lib/changelog/github-client";
import { getCurrentRelease } from "@/lib/release/current-release";
import { createPageMetadata, siteConfig } from "@/lib/site-config";
import type { Metadata } from "next";

const homeTitle = "CompozyOS — The autonomous control plane for your agents";

export const metadata: Metadata = {
  ...createPageMetadata({
    title: homeTitle,
    description: siteConfig.description,
    path: "/",
  }),
  title: {
    absolute: homeTitle,
  },
};

export default async function HomePage() {
  const [release, releases] = await Promise.all([getCurrentRelease(), loadChangelogReleases()]);
  const releaseCount =
    releases.status === "ready" && releases.releases.length > 0 ? releases.releases.length : null;
  return (
    // The draft inherits the app's 13.5px/1.5 body; the marketing shell's base is larger.
    <div className="text-[13.5px] leading-normal tracking-[-0.01em]">
      <WebSiteJsonLd />
      <Hero />
      <Providers />
      <Problem />
      <AlwaysOn />
      <UseCases />
      <Community releases={releaseCount} />
      <Supervise />
      <Orchestrate />
      <BuiltIn />
      <Loops />
      <Automations />
      <Extensibility />
      <Comparison />
      <Install releaseTag={release.tag} />
      <FinalCta />
    </div>
  );
}
