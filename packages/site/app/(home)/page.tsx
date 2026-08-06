import {
  AutonomyKernelSection,
  BentoSection,
  BridgesSection,
  Comparison,
  ExtensibilitySection,
  FeaturesSection,
  FinalCta,
  Hero,
  InstallSection,
  MemoryDreamSection,
  NetworkSection,
  SupportedAgents,
} from "@/components/landing";
import { WebSiteJsonLd } from "@/components/seo/structured-data";
import { getCurrentRelease } from "@/lib/release/current-release";
import { goInstallCommand } from "@/lib/release/install-commands";
import { createPageMetadata, siteConfig } from "@/lib/site-config";
import type { Metadata } from "next";

const homeTitle = "CompozyOS — Agent operating system for real work";

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
  const release = await getCurrentRelease();
  return (
    <>
      <WebSiteJsonLd />
      <Hero />
      <BentoSection />
      <MemoryDreamSection />
      <AutonomyKernelSection />
      <FeaturesSection />
      <ExtensibilitySection />
      <SupportedAgents />
      <NetworkSection />
      <BridgesSection />
      <InstallSection goInstallCommand={goInstallCommand(release.tag)} />
      <Comparison />
      <FinalCta />
    </>
  );
}
