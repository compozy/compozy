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
import { siteConfig } from "@/lib/site-config";

export const metadata = {
  title: {
    absolute: "CompozyOS — Agent operating system for real work",
  },
  description: siteConfig.description,
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
