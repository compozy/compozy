import "./landing.css";

import { LazyMotion, domMax } from "motion/react";
import { definePrototype } from "@compozy/proto";

import { BuiltIn } from "./built-in";
import { Comparison, FinalCta, Install, SiteFooter } from "./sections-close";
import { AlwaysOn, Problem } from "./sections-story";
import { Hero, Providers, SiteHeader } from "./sections-open";
import { Community, Orchestrate, Supervise, UseCases } from "./sections-proof";
import { Automations, Extensibility, Loops } from "./sections-run";

export const meta = definePrototype({
  title: "Landing page redesign",
  spec: "landing-redesign",
  surface: "compozy.com home",
  states: [{ id: "default", label: "Landing" }],
});

export default function Prototype() {
  return (
    <LazyMotion features={domMax}>
      <div id="landing" className="dark bg-[#0a0a0a] font-sans text-fg antialiased">
        <SiteHeader />
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
          <Comparison />
          <Install />
          <FinalCta />
        </main>
        <SiteFooter />
      </div>
    </LazyMotion>
  );
}
