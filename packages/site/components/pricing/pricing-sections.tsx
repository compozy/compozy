"use client";

import { Eyebrow, Pill, cn } from "@compozy/ui";
import { ArrowRight, Check, ChevronDown, Download, Minus } from "@compozy/ui/icons";
import { useState } from "react";

import { ASSET } from "@/components/landing/assets";
import { CONTAINER, FOCUS, HAIRLINE } from "@/components/site/layout";
import {
  H1,
  H2,
  H3,
  InstallCommand,
  LandingEyebrow,
  Lead,
  Muted,
  Plate,
  PrimaryCta,
  SecondaryCta,
  Small,
  TextLink,
} from "@/components/site/primitives";
import { VERIFIED_INSTALLER_COMMAND } from "@/lib/release/install-commands";
import {
  COMPARISON_GROUPS,
  FREE_FEATURES,
  PRICING_FAQ,
  PRO_FEATURES,
  proUpgradeUrl,
  type BillingInterval,
} from "./pricing-data";

export function BillingSwitch({
  interval,
  onChange,
}: {
  interval: BillingInterval;
  onChange: (next: BillingInterval) => void;
}) {
  return (
    <div
      role="group"
      aria-label="Billing interval"
      className="inline-flex items-center gap-1 rounded-[14px] border border-site-light-120 bg-site-dark-400 p-1 backdrop-blur-md"
    >
      <button
        type="button"
        aria-pressed={interval === "year"}
        onClick={() => onChange("year")}
        className={cn(
          "inline-flex h-10 cursor-pointer items-center gap-2 rounded-[10px] px-4 text-[14px] transition-colors",
          FOCUS,
          interval === "year"
            ? "bg-site-light-100 font-medium text-fg"
            : "text-muted-foreground hover:text-fg"
        )}
      >
        <span>Annual</span>
        <Pill form="tint" size="xs" tone="brand">
          Save 33%
        </Pill>
      </button>
      <button
        type="button"
        aria-pressed={interval === "month"}
        onClick={() => onChange("month")}
        className={cn(
          "inline-flex h-10 cursor-pointer items-center rounded-[10px] px-4 text-[14px] transition-colors",
          FOCUS,
          interval === "month"
            ? "bg-site-light-100 font-medium text-fg"
            : "text-muted-foreground hover:text-fg"
        )}
      >
        Monthly
      </button>
    </div>
  );
}

export function PricingHero({
  interval,
  onIntervalChange,
}: {
  interval: BillingInterval;
  onIntervalChange: (next: BillingInterval) => void;
}) {
  const proCheckoutUrl = proUpgradeUrl(interval);

  return (
    <section className="relative overflow-x-clip pb-20 md:pb-28">
      <Plate
        src={ASSET.plateWatch}
        className="inset-x-0 top-0 h-[560px] w-full object-[50%_30%] opacity-65 [mask-image:linear-gradient(180deg,black_45%,transparent)]"
      />
      <div className={cn(CONTAINER, "relative z-10 pt-32 md:pt-40")}>
        <div className="mx-auto max-w-[780px] text-center">
          <LandingEyebrow>Plans</LandingEyebrow>
          <div className="[&_h1]:md:text-[72px] [&_h1]:md:leading-[1.02]">
            <H1>Free on your computer. Pro across all of them.</H1>
          </div>
          <Lead className="mx-auto max-w-[620px] md:text-[19px]">
            Every local feature and one remote computer are free. Upgrade to Pro for unlimited
            computers, hosted routes from any network, and phone access.
          </Lead>
          <div className="mt-9 flex justify-center">
            <BillingSwitch interval={interval} onChange={onIntervalChange} />
          </div>
        </div>

        <h2 className="sr-only">Plans</h2>

        {/* Two plan columns: Pro stacks first on narrow screens (`_uiux.md` U11), Free left / Pro right on lg+. */}
        <div className="mx-auto mt-12 grid max-w-[980px] gap-6 lg:mt-14 lg:grid-cols-2">
          <article className="order-2 flex flex-col justify-between rounded-[22px] border border-site-light-100 bg-[#101010] p-7 sm:p-9 lg:order-1">
            <div>
              <H3 className="text-[26px] md:text-[30px]">Free</H3>
              <Muted className="mt-2.5 text-[15px]">
                Every local feature and your first remote machine over LAN or SSH.
              </Muted>

              <div className="mt-8 border-t border-site-light-080 pt-7">
                <div className="flex items-baseline gap-2">
                  <span className="font-display text-[44px] leading-none tracking-[-0.03em] text-fg tabular-nums sm:text-[52px]">
                    USD 0
                  </span>
                  <span className="text-[15px] text-muted-foreground">forever</span>
                </div>
                <p className="m-0 mt-2.5 font-mono text-[13px] text-subtle">
                  No credit card required. Open source (MIT).
                </p>
              </div>

              <div className="mt-7">
                <SecondaryCta icon={Download} href="/download" className="w-full">
                  Download
                </SecondaryCta>
              </div>

              <ul className="m-0 mt-8 list-none space-y-3.5 border-t border-site-light-080 p-0 pt-7">
                {FREE_FEATURES.map(feature => (
                  <li
                    key={feature}
                    className="flex items-start gap-3 text-[15px]/[1.45] text-fg/85"
                  >
                    <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
                    <span>{feature}</span>
                  </li>
                ))}
              </ul>
            </div>
          </article>

          <article className="relative order-1 flex flex-col justify-between overflow-hidden rounded-[22px] border border-brand-dim bg-[#11141c] p-7 shadow-[0_32px_90px_-36px_rgba(31,135,252,0.45)] sm:p-9 lg:order-2">
            <span
              aria-hidden
              className="pointer-events-none absolute inset-x-0 top-0 h-px bg-[linear-gradient(90deg,transparent,#1f87fc_35%,#1df0d7_65%,transparent)]"
            />
            <div>
              <H3 className="text-[26px] md:text-[30px]">Pro</H3>
              <Muted className="mt-2.5 text-[15px]">
                Unlimited remote computers, hosted routes, phone access, and hosted webhooks.
              </Muted>

              <div className="mt-8 border-t border-site-light-100 pt-7">
                {interval === "year" ? (
                  <>
                    <div className="flex flex-wrap items-baseline">
                      <span className="font-display text-[44px] leading-none tracking-[-0.03em] text-fg tabular-nums sm:text-[52px]">
                        USD 6.67
                      </span>
                      <span className="text-[15px] text-muted-foreground">
                        /month, billed yearly
                      </span>
                    </div>
                    <p className="m-0 mt-2.5 font-mono text-[13px] text-brand-strong tabular-nums">
                      USD 79.99/year
                    </p>
                  </>
                ) : (
                  <>
                    <div className="flex flex-wrap items-baseline">
                      <span className="font-display text-[44px] leading-none tracking-[-0.03em] text-fg tabular-nums sm:text-[52px]">
                        USD 9.99
                      </span>
                      <span className="text-[15px] text-muted-foreground">/month</span>
                    </div>
                    <p className="m-0 mt-2.5 font-mono text-[13px] text-subtle">
                      Switch to annual billing anytime for USD 79.99/year.
                    </p>
                  </>
                )}
              </div>

              <div className="mt-7">
                <PrimaryCta icon={ArrowRight} href={proCheckoutUrl} className="w-full">
                  Get Pro
                </PrimaryCta>
              </div>

              <div className="mt-8 border-t border-site-light-100 pt-7">
                <p className="m-0 mb-3.5 text-[13.5px] font-medium text-fg/90">
                  Everything in Free, plus:
                </p>
                <ul className="m-0 list-none space-y-3.5 p-0">
                  {PRO_FEATURES.map(feature => (
                    <li
                      key={feature}
                      className="flex items-start gap-3 text-[15px]/[1.45] text-fg/90"
                    >
                      <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-brand-strong" />
                      <span>{feature}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          </article>
        </div>

        <div className="mx-auto mt-7 max-w-[980px] text-center">
          <Small className="text-[13.5px] text-muted-foreground">
            Prices in USD. Plus applicable taxes, calculated at checkout.
          </Small>
          <Small className="mt-1.5 text-[13px] text-subtle">
            Promotion codes are entered at checkout. Full refund within 14 days of first payment or
            annual renewal.
          </Small>
        </div>
      </div>
    </section>
  );
}

export function PricingComparison() {
  return (
    <section
      id="comparison"
      className={cn("scroll-mt-20 border-t bg-[#070707] py-14 md:py-20", HAIRLINE)}
    >
      <div className={CONTAINER}>
        <div className="max-w-[680px]">
          <H2 size="sm">What is free and what is Pro.</H2>
          <Lead className="mt-3 text-[16.5px] md:text-[18px]">
            Your code and sessions stay on your own machines on every plan. Pro pays for unlimited
            remote connections, hosted routes, and cloud infrastructure.
          </Lead>
        </div>

        <div className="mt-8 overflow-hidden rounded-[22px] border border-site-light-100 bg-[#0c0c0c] md:mt-10">
          <div
            className={cn(
              "hidden grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_minmax(0,1fr)] items-center gap-6 border-b bg-[#101010] px-7 py-4 md:grid",
              HAIRLINE
            )}
          >
            <span className="font-mono text-[12px] text-subtle">Capability</span>
            <span className="text-[14.5px] font-medium text-fg">Free</span>
            <span className="text-[14.5px] font-medium text-brand-strong">Pro</span>
          </div>

          <div className="divide-y divide-site-light-080">
            {COMPARISON_GROUPS.map(group => (
              <div key={group.id} className="px-5 py-7 sm:px-7">
                <Eyebrow className="mb-5 block text-subtle" variant="caps">
                  {group.title}
                </Eyebrow>
                <ul className="m-0 list-none divide-y divide-site-light-060 p-0">
                  {group.rows.map(row => (
                    <li
                      key={row.label}
                      className="grid gap-3 py-4 first:pt-0 last:pb-0 md:grid-cols-[minmax(0,1.4fr)_minmax(0,1fr)_minmax(0,1fr)] md:items-center md:gap-6"
                    >
                      <div className="min-w-0">
                        <p className="m-0 text-[15px]/[1.4] text-fg">{row.label}</p>
                        {row.detail ? (
                          <p className="m-0 mt-1 text-[13.5px]/[1.45] text-subtle">{row.detail}</p>
                        ) : null}
                      </div>

                      <div className="grid grid-cols-2 gap-4 md:contents">
                        <div className="flex items-center gap-2 text-[14px]">
                          <span className="font-mono text-[11.5px] text-subtle md:hidden">
                            Free:
                          </span>
                          {row.free === "Not included" ? (
                            <span className="inline-flex items-center gap-1.5 text-subtle">
                              <Minus aria-hidden className="size-3.5" />
                              Not included
                            </span>
                          ) : (
                            <span className="text-muted-foreground">{row.free}</span>
                          )}
                        </div>

                        <div className="flex items-center gap-2 text-[14px]">
                          <span className="font-mono text-[11.5px] text-brand-strong md:hidden">
                            Pro:
                          </span>
                          <span
                            className={cn(
                              "inline-flex items-center gap-1.5",
                              row.proEmphasis ? "font-medium text-fg" : "text-fg/85"
                            )}
                          >
                            {row.proEmphasis ? (
                              <Check aria-hidden className="size-3.5 shrink-0 text-brand-strong" />
                            ) : null}
                            {row.pro}
                          </span>
                        </div>
                      </div>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}

export function PricingFaq({ initialOpenId = null }: { initialOpenId?: string | null }) {
  const [openId, setOpenId] = useState<string | null>(initialOpenId);

  return (
    <section id="faq" className={cn("scroll-mt-20 border-t py-14 md:py-24", HAIRLINE)}>
      <div className={cn(CONTAINER, "grid gap-12 lg:grid-cols-[340px_minmax(0,1fr)] lg:gap-16")}>
        <div>
          <H2 size="sm">Questions</H2>
          <Muted className="mt-4 max-w-[300px]">
            Straight answers on sign-in, limits, cancellation, refunds, and where your code runs.
          </Muted>
          <p className="m-0 mt-6 text-[14px] text-subtle">
            Something else? Write to{" "}
            <a
              href="mailto:support@compozy.com"
              className="text-fg underline decoration-site-light-250 underline-offset-4 transition-colors hover:decoration-fg"
            >
              support@compozy.com
            </a>
            .
          </p>
        </div>

        <div className="divide-y divide-site-light-080 border-y border-site-light-080">
          {PRICING_FAQ.map(item => {
            const open = openId === item.id;
            return (
              <div key={item.id} id={`faq-${item.id}`} className="py-1">
                <button
                  type="button"
                  aria-expanded={open}
                  onClick={() => setOpenId(current => (current === item.id ? null : item.id))}
                  className={cn(
                    "flex w-full cursor-pointer items-center justify-between gap-6 py-5 text-left text-[17px]/[1.35] font-normal text-fg transition-colors hover:text-fg-strong sm:text-[18.5px]",
                    FOCUS
                  )}
                >
                  <span>{item.question}</span>
                  <ChevronDown
                    aria-hidden
                    className={cn(
                      "size-4 shrink-0 text-subtle transition-transform duration-200",
                      open && "rotate-180 text-fg"
                    )}
                  />
                </button>
                {open ? (
                  <div className="space-y-3.5 pr-8 pb-6">
                    {item.answer.map(paragraph => (
                      <Muted key={paragraph} className="text-[15.5px]/[1.6]">
                        {paragraph}
                      </Muted>
                    ))}
                    {item.link ? (
                      <div className="pt-1">
                        <TextLink href={item.link.href} className="text-[14.5px]">
                          {item.link.label}
                        </TextLink>
                      </div>
                    ) : null}
                  </div>
                ) : null}
              </div>
            );
          })}
        </div>
      </div>
    </section>
  );
}

export function PricingClosingCta({ interval }: { interval: BillingInterval }) {
  const proCheckoutUrl = proUpgradeUrl(interval);
  return (
    <section className={cn("border-t bg-[#070707] py-20 md:py-28", HAIRLINE)}>
      <div className={CONTAINER}>
        <div className="flex flex-col justify-between gap-10 lg:flex-row lg:items-end">
          <div className="max-w-[640px]">
            <H2 size="sm">Start free on your computer.</H2>
            <Lead className="mt-4 text-[17.5px] md:text-[19px]">
              Install CompozyOS, sign in, and add your first remote computer in minutes. Upgrade to
              Pro whenever you need more machines or hosted routes.
            </Lead>
          </div>
          <div className="flex flex-col items-start gap-3.5 lg:items-end">
            <div className="flex flex-wrap gap-3">
              <PrimaryCta icon={ArrowRight} href={proCheckoutUrl}>
                Get Pro
              </PrimaryCta>
              <SecondaryCta icon={Download} href="/download">
                Download
              </SecondaryCta>
            </div>
            <InstallCommand command={VERIFIED_INSTALLER_COMMAND} />
          </div>
        </div>
      </div>
    </section>
  );
}
