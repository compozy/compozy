import { Logo, Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { Check, Download, Star } from "@compozy/ui/icons";
import { m, useReducedMotion, useScroll, useTransform } from "motion/react";
import { useRef } from "react";

import final from "./assets/plate-final.jpg";
import { Dust } from "./fx/dust";
import { COMPARE_COLUMNS, COMPARE_ROWS, INSTALL_COMMAND, INSTALL_METHODS, NAV } from "./content";
import { Reveal, ScrollWords } from "./motion";
import { useIsDesktop } from "./hooks/use-is-desktop";
import { CONTAINER, HAIRLINE } from "./layout";
import {
  InstallCommand,
  LandingEyebrow,
  H2,
  Lead,
  Muted,
  Plate,
  PrimaryCta,
  SecondaryCta,
  TextLink,
} from "./primitives";
import { Typed } from "./sections-run";

export function Comparison() {
  return (
    <section id="comparison" className="relative bg-[#f4f4f1] py-24 text-[#0a0a0a] md:py-44">
      <div className={CONTAINER}>
        <div className="max-w-[860px]">
          <p className="m-0 mb-6 inline-flex rounded-pill border border-[color:var(--dark-150)] px-3.5 py-1.5 font-mono text-[12.5px]">
            Side by side
          </p>
          <h2 className="m-0 font-display text-[38px]/[1.06] font-normal tracking-[-0.03em] md:text-[80px]/[1]">
            Every piece, side by side.
          </h2>
          <p className="m-0 mt-6 text-[18px]/[1.5] text-[color:var(--dark-600)] md:text-[21px]/[1.45]">
            Synara, T3 Code, Hermes, and OpenClaw each prove demand for part of the system.
            CompozyOS ships those parts in one product.
          </p>
        </div>
        <div className="mt-14 overflow-x-auto md:mt-20">
          <table className="w-full min-w-[860px] border-collapse text-left text-[16px]">
            <thead>
              <tr>
                <th className="w-[34%] pb-5 font-normal" />
                <th className="rounded-t-[16px] bg-[#2448e6] px-4 py-5 text-center font-medium text-white">
                  CompozyOS
                </th>
                {COMPARE_COLUMNS.map(column => (
                  <th
                    key={column}
                    className="px-4 pb-5 text-center font-normal text-[color:var(--dark-550)]"
                  >
                    {column}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {COMPARE_ROWS.map((row, index) => (
                <tr
                  key={row}
                  className="border-t border-[color:var(--dark-100)] transition-colors hover:bg-[color:var(--dark-030)]"
                >
                  <td className="py-6 pr-6 text-[#0a0a0a]">{row}</td>
                  <td
                    className={cn(
                      "bg-[#2448e6] px-4 text-center",
                      index === COMPARE_ROWS.length - 1 && "rounded-b-[16px]"
                    )}
                  >
                    <m.span
                      className="inline-block"
                      initial={{ scale: 0 }}
                      whileInView={{ scale: 1 }}
                      viewport={{ once: true }}
                      transition={{
                        delay: 0.1 * index,
                        type: "spring",
                        stiffness: 320,
                        damping: 14,
                      }}
                    >
                      <Check aria-label="Yes" className="size-5 text-white" />
                    </m.span>
                  </td>
                  {COMPARE_COLUMNS.map(column => (
                    <td
                      key={column}
                      className="px-4 text-center font-mono text-[12px] text-[color:var(--dark-400)]"
                    >
                      to verify
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="m-0 mt-10 max-w-[760px] text-[16px]/[1.5] text-[color:var(--dark-700)]">
          Why it holds: one runtime, one state model. Sessions, agents, approvals, and runs are core
          objects, so nothing has to be glued together.
        </p>
        <p className="m-0 mt-3 text-[13px] text-[color:var(--dark-450)]">
          Checked against each product's public documentation on &lt;date&gt;.
        </p>
      </div>
    </section>
  );
}

const TAB =
  "h-11 rounded-[12px] px-4 text-[14.5px] after:hidden data-[active]:bg-[color:var(--light-100)]";

export function Install() {
  return (
    <section id="install" className="relative py-24 md:py-44">
      <div className={cn(CONTAINER, "grid gap-14 lg:grid-cols-[1fr_1.15fr] lg:gap-20")}>
        <div>
          <Reveal>
            <LandingEyebrow>Getting started</LandingEyebrow>
          </Reveal>
          <H2>
            <ScrollWords text="Install CompozyOS. Start one session." />
          </H2>
          <Reveal>
            <Lead>
              Download the desktop app, or install the runtime with the verified installer,
              Homebrew, npm, or Go. Today's experience is technical: a terminal, a local daemon, and
              one config file.
            </Lead>
          </Reveal>
        </div>
        <Reveal>
          <div className="overflow-hidden rounded-[22px] border border-[color:var(--light-120)] bg-[#101010] shadow-[0_0_120px_-40px_rgba(31,135,252,0.6)]">
            <Tabs defaultValue="Desktop" className="gap-0">
              <div className="border-b border-[color:var(--light-100)] p-2">
                <TabsList className="flex-wrap gap-1 border-b-0 group-data-horizontal/tabs:h-auto">
                  {["Desktop", ...INSTALL_METHODS.map(method => method.id)].map(id => (
                    <TabsTrigger key={id} value={id} className={TAB}>
                      {id}
                    </TabsTrigger>
                  ))}
                </TabsList>
              </div>
              <TabsContent
                value="Desktop"
                className="flex min-h-[132px] flex-wrap items-center gap-6 p-7"
              >
                <PrimaryCta icon={Download}>Download for macOS</PrimaryCta>
                <TextLink>All platforms and checksums</TextLink>
              </TabsContent>
              {INSTALL_METHODS.map(method => (
                <TabsContent key={method.id} value={method.id} className="min-h-[132px] p-7">
                  <InstallCommand command={method.command} className="w-full justify-between" />
                </TabsContent>
              ))}
            </Tabs>
            <ol className="m-0 list-none border-t border-[color:var(--light-100)] p-0">
              <li className="flex gap-4 p-7">
                <span className="grid size-8 shrink-0 place-items-center rounded-full bg-brand font-mono text-[13px] text-brand-ink">
                  1
                </span>
                <div>
                  <p className="m-0 text-[17px] text-fg">Start the daemon</p>
                  <p className="m-0 mt-2 font-mono text-[15px] text-brand-strong">
                    <Typed text="compozy daemon start" />
                  </p>
                </div>
              </li>
              <li className="flex gap-4 border-t border-[color:var(--light-100)] p-7">
                <span className="grid size-8 shrink-0 place-items-center rounded-full bg-[color:var(--light-100)] font-mono text-[13px] text-fg">
                  2
                </span>
                <div>
                  <p className="m-0 text-[17px] text-fg">Open a session</p>
                  <Muted className="mt-1">from the repository you want to work on</Muted>
                </div>
              </li>
            </ol>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export function FinalCta() {
  const ref = useRef<HTMLElement>(null);
  const reduced = useReducedMotion();
  const desktop = useIsDesktop();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end end"] });
  const scale = useTransform(scrollYProgress, [0, 1], reduced ? [1, 1] : [1.18, 1]);
  return (
    <section
      ref={ref}
      id="final"
      className={cn("relative overflow-hidden border-t py-36 md:py-60", HAIRLINE)}
    >
      <Plate
        src={final}
        style={{ scale, transformOrigin: "100% 100%" }}
        className="inset-0 size-full [mask-image:linear-gradient(180deg,transparent,black_18%,black_80%,transparent)]"
      />
      {desktop && !reduced ? <Dust count={120} className="absolute inset-0 size-full" /> : null}
      <div className={cn(CONTAINER, "relative")}>
        <div className="max-w-[860px]">
          <H2 size="xl">
            <ScrollWords text="Keep every agent running." />
          </H2>
          <Reveal>
            <Lead className="max-w-[580px] text-fg/80">
              See every session, let always-on agents watch your projects, and step in only when one
              needs you.
            </Lead>
            <div className="mt-10 flex flex-wrap gap-3">
              <PrimaryCta icon={Download}>Download CompozyOS</PrimaryCta>
              <SecondaryCta icon={Star}>Star on GitHub</SecondaryCta>
            </div>
            <InstallCommand command={INSTALL_COMMAND} className="mt-4" />
          </Reveal>
        </div>
      </div>
    </section>
  );
}

export function SiteFooter() {
  return (
    <footer className={cn("border-t", HAIRLINE)}>
      <div
        className={cn(
          CONTAINER,
          "flex flex-col gap-6 py-9 md:flex-row md:items-center md:justify-between"
        )}
      >
        <Logo variant="logo" className="h-[22px] self-start md:self-auto" />
        <nav className="flex flex-wrap gap-x-7 gap-y-3">
          {[...NAV, "GitHub"].map(item => (
            <a
              key={item}
              href="#"
              className="text-[14px] text-muted-foreground transition-colors hover:text-fg"
            >
              {item}
            </a>
          ))}
        </nav>
        <p className="m-0 text-[14px] text-muted-foreground">Keep every agent running.</p>
      </div>
    </footer>
  );
}
