import CompozyBentoGrid from "@/components/bento/bento-grid";
import FeaturesSection from "@/components/lp/features-section";
import { Footer } from "@/components/lp/footer";
import HeroSection from "@/components/lp/hero-section";
import { Pricing } from "@/components/lp/pricing";

export default function HomePage() {
  return (
    <>
      <HeroSection />
      <FeaturesSection />
      <CompozyBentoGrid />
      <Pricing />
      <Footer />
    </>
  );
}
