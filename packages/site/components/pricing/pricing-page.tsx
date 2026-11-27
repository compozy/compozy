"use client";

import { useState } from "react";

import type { BillingInterval } from "./pricing-data";
import { PricingClosingCta, PricingComparison, PricingFaq, PricingHero } from "./pricing-sections";

export function PricingPageContent({
  initialInterval = "year",
  initialOpenFaqId = null,
}: {
  initialInterval?: BillingInterval;
  initialOpenFaqId?: string | null;
}) {
  const [interval, setInterval] = useState<BillingInterval>(initialInterval);

  return (
    <div className="text-[13.5px] leading-normal tracking-[-0.01em]">
      <PricingHero interval={interval} onIntervalChange={setInterval} />
      <PricingComparison />
      <PricingFaq initialOpenId={initialOpenFaqId} />
      <PricingClosingCta interval={interval} />
    </div>
  );
}
