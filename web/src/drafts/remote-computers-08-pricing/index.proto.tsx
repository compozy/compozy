import "../landing/landing.css";
import "./site-dark-pin.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { LazyMotion, MotionConfig, domMax } from "motion/react";
import { useLayoutEffect, useState } from "react";

import { SiteFooter } from "../landing/sections-close";
import { SiteHeader } from "../landing/sections-open";
import type { BillingInterval } from "./pricing-data";
import { PricingClosingCta, PricingComparison, PricingFaq, PricingHero } from "./pricing-sections";

export const meta = definePrototype({
  title: "Pricing",
  spec: "remote-computers",
  surface: "U11",
  route: "/pricing",
  states: [
    { id: "annual", label: "Annual billing (default)" },
    { id: "monthly", label: "Monthly billing" },
    { id: "faq-open", label: "FAQ · Why sign-in is required" },
    { id: "faq-refunds", label: "FAQ · Refund policy answer" },
    { id: "comparison", label: "Comparison · Free vs Pro capabilities" },
    { id: "footer-links", label: "Footer · Pricing, Terms, Privacy and Refunds" },
  ],
});

interface PricingPreset {
  readonly interval: BillingInterval;
  readonly openFaqId: string | null;
  readonly scrollTo?: "faq" | "comparison" | "site-footer";
}

const PRESETS: Record<string, PricingPreset> = {
  annual: { interval: "year", openFaqId: null },
  monthly: { interval: "month", openFaqId: null },
  "faq-open": { interval: "year", openFaqId: "why-sign-in", scrollTo: "faq" },
  "faq-refunds": { interval: "year", openFaqId: "refunds", scrollTo: "faq" },
  comparison: { interval: "year", openFaqId: null, scrollTo: "comparison" },
  "footer-links": { interval: "year", openFaqId: null, scrollTo: "site-footer" },
};

function PricingPage({ preset }: { preset: PricingPreset }) {
  const [interval, setInterval] = useState<BillingInterval>(preset.interval);

  useLayoutEffect(() => {
    const scrollTarget = () => {
      if (!preset.scrollTo) {
        window.scrollTo({ top: 0, behavior: "instant" });
        return;
      }
      if (preset.scrollTo === "site-footer") {
        const footer = document.getElementById("site-footer");
        footer?.scrollIntoView({ behavior: "instant", block: "end" });
        const maxTop = Math.max(0, document.documentElement.scrollHeight - window.innerHeight);
        window.scrollTo({ top: maxTop, behavior: "instant" });
        return;
      }
      document
        .getElementById(preset.scrollTo)
        ?.scrollIntoView({ behavior: "instant", block: "start" });
    };
    scrollTarget();
    const frame = requestAnimationFrame(scrollTarget);
    const settle = window.setTimeout(scrollTarget, 50);
    void document.fonts?.ready.then(scrollTarget);
    return () => {
      cancelAnimationFrame(frame);
      window.clearTimeout(settle);
    };
  }, [preset.scrollTo]);

  return (
    <div id="landing" className="rc-site-dark dark bg-[#0a0a0a] font-sans text-fg antialiased">
      <SiteHeader active="Pricing" scrolled={Boolean(preset.scrollTo)} />
      <main>
        <PricingHero interval={interval} onIntervalChange={setInterval} />
        <PricingComparison />
        <PricingFaq initialOpenId={preset.openFaqId} />
        <PricingClosingCta interval={interval} />
      </main>
      <SiteFooter active="Pricing" id="site-footer" />
    </div>
  );
}

export default function RemoteComputersPricingDraft({ state }: PrototypeProps) {
  const preset = PRESETS[state] ?? PRESETS.annual!;
  return (
    <LazyMotion features={domMax}>
      <MotionConfig reducedMotion="always">
        <PricingPage key={state} preset={preset} />
      </MotionConfig>
    </LazyMotion>
  );
}
