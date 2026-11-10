/** Everything else that comes built in: feature tabs over one window, then the smaller details. */
import { cn } from "@compozy/ui";

import { ASSET } from "@/components/landing/assets";
import { DetailsRail } from "@/components/landing/details-rail";
import { FeatureTabs } from "@/components/landing/feature-tabs";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import { Reveal, ScrollWords } from "@/components/landing/motion";
import { H2, LandingEyebrow, Plate } from "@/components/site/primitives";

export function BuiltIn() {
  return (
    <section
      id="features"
      className={cn("relative overflow-x-clip border-t bg-[#070707] py-24 md:py-44", HAIRLINE)}
    >
      <Plate
        src={ASSET.plateTerrain}
        className="inset-x-0 top-[420px] h-[900px] w-full opacity-80 [mask-image:linear-gradient(180deg,transparent,black_30%,black_70%,transparent)]"
      />
      <div className={cn(CONTAINER, "relative")}>
        <div className="text-center">
          <Reveal>
            <LandingEyebrow>Built in</LandingEyebrow>
          </Reveal>
          <H2 size="lg">
            <ScrollWords text="Comes with what you would otherwise build." />
          </H2>
        </div>
        <FeatureTabs />
      </div>
      <DetailsRail />
    </section>
  );
}
