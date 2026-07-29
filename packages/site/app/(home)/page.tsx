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

export default function HomePage() {
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
      <InstallSection />
      <Comparison />
      <FinalCta />
    </>
  );
}
