"use client";

/**
 * Landing section "Your agents, on all your computers" (`remote-computers-09-computers-section`,
 * ADR-014 §2, `_uiux.md` U11). Rebuilds the three approved showcase tabs (Computers, Run on, Phone
 * access) with `@compozy/ui` primitives and site tokens.
 */
import { Tabs, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { Globe, Route, ServerIcon, ShieldCheck, Smartphone, Terminal } from "@compozy/ui/icons";
import { useState } from "react";

import { ASSET } from "@/components/landing/assets";
import type { ShowcaseTab } from "@/components/landing/computers-data";
import { RunOnShot } from "@/components/landing/computers-run-on-shot";
import { ComputersListShot, PhoneShot } from "@/components/landing/computers-shots";
import { HREF } from "@/components/landing/content";
import { CONTAINER, HAIRLINE } from "@/components/site/layout";
import {
  H2,
  LandingEyebrow,
  Muted,
  Plate,
  ProductWindow,
  TextLink,
  type Icon,
} from "@/components/site/primitives";

const SHOWCASE_TABS: ReadonlyArray<{
  readonly id: ShowcaseTab;
  readonly label: string;
  readonly icon: Icon;
}> = [
  { id: "computers", label: "Computers", icon: ServerIcon },
  { id: "run-on", label: "Run on", icon: Route },
  { id: "phone", label: "Phone access", icon: Smartphone },
];

const COMPUTERS_HIGHLIGHTS: ReadonlyArray<{
  readonly icon: Icon;
  readonly title: string;
  readonly body: string;
}> = [
  {
    icon: Terminal,
    title: "No manual daemon setup",
    body: "Pick any host from your SSH config. CompozyOS verifies the checksum and starts the background service for you.",
  },
  {
    icon: Globe,
    title: "Automatic route selection",
    body: "Uses LAN when you are on the same network, SSH when configured, and a Pro hosted route from anywhere else.",
  },
  {
    icon: ShieldCheck,
    title: "Sessions stay on your machines",
    body: "Code, prompts, and transcripts stay on the computer running the agent. Compozy Cloud never sees session traffic.",
  },
];

export function Computers({ initialTab = "computers" }: { initialTab?: ShowcaseTab }) {
  const [active, setActive] = useState<ShowcaseTab>(initialTab);

  return (
    <section
      id="computers"
      className={cn(
        "relative scroll-mt-16 overflow-x-clip border-t bg-[#070707] py-16 md:py-24",
        HAIRLINE
      )}
    >
      <Plate
        src={ASSET.plateOrder}
        className="top-0 right-[-8%] h-[280px] w-[48%] opacity-30 [mask-image:linear-gradient(90deg,transparent,black_55%),linear-gradient(180deg,black_35%,transparent)] [mask-composite:intersect]"
      />

      <div className={cn(CONTAINER, "relative")}>
        <Tabs
          value={active}
          onValueChange={value => setActive(value as ShowcaseTab)}
          className="gap-0"
        >
          <div className="grid items-end gap-5 lg:grid-cols-[minmax(0,1.2fr)_minmax(0,0.9fr)] lg:gap-10">
            <div>
              <LandingEyebrow icon={ServerIcon} className="mb-3.5">
                Remote computers
              </LandingEyebrow>
              <H2 size="sm">Your agents, on all your computers.</H2>
            </div>

            <div className="flex flex-col gap-4 lg:items-end">
              <Muted className="max-w-[520px] text-[15.5px]/[1.5] lg:text-right">
                Add a Linux box or desktop over SSH in one dialog, pick where each new session runs,
                and check in from your phone when you step away.
              </Muted>
              <TabsList className="h-auto w-fit flex-wrap gap-1 rounded-[14px] border border-site-light-120 bg-[#111111] p-1 group-data-horizontal/tabs:h-auto">
                {SHOWCASE_TABS.map(tab => {
                  const isSelected = tab.id === active;
                  return (
                    <TabsTrigger
                      key={tab.id}
                      value={tab.id}
                      className={cn(
                        "h-9 gap-2 rounded-[10px] px-3.5 text-[13.5px] after:hidden",
                        isSelected && "bg-site-light-100 text-fg"
                      )}
                    >
                      <span className="inline-flex items-center gap-1.5">
                        <tab.icon
                          className={cn("size-3.5 shrink-0", isSelected && "text-brand-strong")}
                          aria-hidden
                        />
                        {tab.label}
                      </span>
                    </TabsTrigger>
                  );
                })}
              </TabsList>
            </div>
          </div>

          <ProductWindow className="mt-6" aspect="min-h-[540px] lg:h-[550px]">
            {active === "computers" ? <ComputersListShot /> : null}
            {active === "run-on" ? <RunOnShot /> : null}
            {active === "phone" ? <PhoneShot /> : null}
          </ProductWindow>
        </Tabs>

        <ul className="m-0 mt-10 grid list-none gap-8 p-0 md:grid-cols-3">
          {COMPUTERS_HIGHLIGHTS.map(item => (
            <li key={item.title} className="space-y-2">
              <item.icon className="size-5 text-brand-strong" aria-hidden />
              <p className="m-0 text-[16.5px] font-medium text-fg">{item.title}</p>
              <Muted className="text-[14.5px]/[1.55]">{item.body}</Muted>
            </li>
          ))}
        </ul>

        <div
          className={cn(
            "mt-10 flex flex-wrap items-center justify-between gap-6 border-t pt-7",
            HAIRLINE
          )}
        >
          <Muted className="max-w-[540px] text-[15px]">
            Free includes one remote computer over LAN or SSH. Pro adds unlimited computers, hosted
            routes, and phone access.
          </Muted>
          <div className="flex flex-wrap items-center gap-7">
            <TextLink href={HREF.computers}>Read the Computers docs</TextLink>
            <TextLink href={HREF.pricing}>See pricing</TextLink>
          </div>
        </div>
      </div>
    </section>
  );
}
