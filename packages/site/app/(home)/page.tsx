import {
  Comparison,
  ExtensibilitySection,
  FeatureWall,
  FinalCta,
  Hero,
  ProofSection,
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
      <ExtensibilitySection />
      <FeatureWall />
      <Comparison />
      <ProofSection />
      <FinalCta />
    </>
  );
}
