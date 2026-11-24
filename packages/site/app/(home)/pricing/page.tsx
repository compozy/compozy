import type { Metadata } from "next";

import { PricingPageContent } from "@/components/pricing/pricing-page";
import { createPageMetadata } from "@/lib/site-config";

export const metadata: Metadata = createPageMetadata({
  title: "Pricing",
  description:
    "Every local feature and one remote computer are free. Upgrade to Compozy Pro for unlimited remote computers, hosted routes, phone access, and hosted webhooks.",
  path: "/pricing",
  keywords: ["CompozyOS pricing", "Compozy Pro", "remote computers", "hosted route"],
});

export default function PricingPage() {
  return <PricingPageContent />;
}
