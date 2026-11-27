import "../landing/landing.css";
import "../remote-computers-08-pricing/site-dark-pin.css";

import { definePrototype, type PrototypeProps } from "@protohq/cli";
import { LazyMotion, MotionConfig, domMax } from "motion/react";
import { useLayoutEffect } from "react";

import { BuiltIn } from "../landing/built-in";
import { FinalCta, SiteFooter } from "../landing/sections-close";
import { Hero, Providers, SiteHeader } from "../landing/sections-open";
import { Community, Orchestrate, Supervise, UseCases } from "../landing/sections-proof";
import { Automations, Extensibility, Loops } from "../landing/sections-run";
import { AlwaysOn, Problem } from "../landing/sections-story";
import { ComputersLandingSection, type ShowcaseTab } from "./computers-showcase";
import { UpdatedComparison, UpdatedInstall } from "./updated-landing-close";

export const meta = definePrototype({
  title: "Computers on the landing",
  spec: "remote-computers",
  surface: "U11",
  route: "/",
  states: [
    { id: "section", label: "Computers section · Computers list" },
    { id: "section-run-on", label: "Computers section · Run on picker" },
    { id: "section-phone", label: "Computers section · Phone browser view" },
    { id: "section-wide", label: "Computers section · Wide viewport composition" },
    { id: "comparison-row", label: "Comparison · Runs on your computers (MIT)" },
    { id: "install-steps", label: "Install · Sign in step, no go install" },
  ],
});

interface LandingSectionPreset {
  readonly scrollTo: "computers" | "comparison" | "install";
  readonly tab: ShowcaseTab;
  readonly openRunOnPicker: boolean;
}

const PRESETS: Record<string, LandingSectionPreset> = {
  section: { scrollTo: "computers", tab: "computers", openRunOnPicker: false },
  "section-run-on": { scrollTo: "computers", tab: "run-on", openRunOnPicker: true },
  "section-phone": { scrollTo: "computers", tab: "phone", openRunOnPicker: false },
  "section-wide": { scrollTo: "computers", tab: "computers", openRunOnPicker: false },
  "comparison-row": { scrollTo: "comparison", tab: "computers", openRunOnPicker: false },
  "install-steps": { scrollTo: "install", tab: "computers", openRunOnPicker: false },
};

function LandingWithComputers({ preset }: { preset: LandingSectionPreset }) {
  useLayoutEffect(() => {
    const scrollTarget = () => {
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
      <SiteHeader scrolled />
      <main>
        <Hero />
        <Providers />
        <Problem />
        <AlwaysOn />
        <UseCases />
        <Community />
        <Supervise />
        <Orchestrate />
        <BuiltIn />
        <Loops />
        <Automations />
        <Extensibility />
        <ComputersLandingSection initialTab={preset.tab} openRunOnPicker={preset.openRunOnPicker} />
        <UpdatedComparison />
        <UpdatedInstall />
        <FinalCta />
      </main>
      <SiteFooter />
    </div>
  );
}

export default function RemoteComputersLandingSectionDraft({ state }: PrototypeProps) {
  const preset = PRESETS[state] ?? PRESETS.section!;
  return (
    <LazyMotion features={domMax}>
      <MotionConfig reducedMotion="always">
        <LandingWithComputers key={state} preset={preset} />
      </MotionConfig>
    </LazyMotion>
  );
}
