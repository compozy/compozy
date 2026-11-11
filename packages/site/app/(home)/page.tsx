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
import { getCurrentRelease } from "@/lib/release/current-release";
import { countPublishedReleases } from "@/lib/release/release-count";
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
  const [release, published] = await Promise.all([getCurrentRelease(), countPublishedReleases()]);
  const releaseCount = published === 0 ? null : published;
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
