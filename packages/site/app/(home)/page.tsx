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
  title: siteConfig.name,
  description: siteConfig.description,
};

export default function HomePage() {
  return (
    <>
      <WebSiteJsonLd />
      <Hero />
      <SupportedAgents />
      <NetworkSection />
      <BentoSection />
      <MemoryDreamSection />
      <AutonomyKernelSection />
      <FeaturesSection />
      <ExtensibilitySection />
      <BridgesSection />
      <InstallSection />
      <Comparison />
      <FinalCta />
    </>
  );
}
