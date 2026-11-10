/** Closing: the side-by-side comparison, install, and the final call to action. */
import { Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { Check, Download, Star } from "@compozy/ui/icons";

import {
  COMPARE_CHECKED_ON,
  COMPARE_COLUMNS,
  COMPARE_ROWS,
  COMPARE_SOURCES,
  COMPARE_VERDICTS,
  type CompareVerdict,
  HREF,
  INSTALL_COMMAND,
  INSTALL_STEPS,
  installMethods,
} from "@/components/landing/content";
import { FinalScene } from "@/components/landing/final-scene";
import { PopCheck } from "@/components/landing/pop-check";
import { Typed } from "@/components/landing/typed";
import { CONTAINER } from "@/components/site/layout";
import { Reveal, ScrollWords } from "@/components/site/motion";
import {
  H2,
  InstallCommand,
  LandingEyebrow,
  Lead,
  Muted,
  PrimaryCta,
  SecondaryCta,
  TextLink,
} from "@/components/site/primitives";
import { siteConfig } from "@/lib/site-config";

const VERDICT_LABEL: Record<CompareVerdict, string> = {
  yes: "Yes",
  partial: "Partial",
  no: "No",
};

/** A competitor's cell: a check, the word "Partial", or a dash, each named for screen readers. */
function Verdict({ value }: { value: CompareVerdict }) {
  if (value === "yes") {
    return <Check aria-label="Yes" className="mx-auto size-5 text-[#0a0a0a]" />;
  }
  return (
    <span aria-label={VERDICT_LABEL[value]} className="font-mono text-[12px] text-site-dark-450">
      {value === "partial" ? "Partial" : "—"}
    </span>
  );
}

export function Comparison() {
  return (
    <section id="comparison" className="relative bg-[#f4f4f1] py-24 text-[#0a0a0a] md:py-44">
      <div className={CONTAINER}>
        <div className="max-w-[860px]">
          <p className="m-0 mb-6 inline-flex rounded-pill border border-site-dark-150 px-3.5 py-1.5 font-mono text-[12.5px]">
            Side by side
          </p>
          <h2 className="m-0 font-display text-[38px]/[1.06] font-normal tracking-[-0.03em] md:text-[80px]/[1]">
            Every piece, side by side.
          </h2>
          <p className="m-0 mt-6 text-[18px]/[1.5] text-site-dark-600 md:text-[21px]/[1.45]">
            Synara, T3 Code, Hermes, and OpenClaw each prove demand for part of the system.
            CompozyOS ships those parts in one product.
          </p>
        </div>
        <div className="mt-14 overflow-x-auto md:mt-20">
          <table className="w-full min-w-[860px] border-collapse text-left text-[16px] leading-[normal]">
            <thead>
              <tr>
                <th className="w-[34%] pb-5 font-normal" />
                <th className="rounded-t-[16px] bg-[#2448e6] px-4 py-5 text-center font-medium text-white">
                  CompozyOS
                </th>
                {COMPARE_COLUMNS.map(column => (
                  <th key={column} className="px-4 pb-5 text-center font-normal text-site-dark-550">
                    {column}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {COMPARE_ROWS.map((row, index) => (
                <tr
                  key={row}
                  className="border-t border-site-dark-100 transition-colors hover:bg-site-dark-030"
                >
                  <td className="py-6 pr-6 text-[#0a0a0a]">{row}</td>
                  <td
                    className={cn(
                      "bg-[#2448e6] px-4 text-center",
                      index === COMPARE_ROWS.length - 1 && "rounded-b-[16px]"
                    )}
                  >
                    <PopCheck index={index} />
                  </td>
                  {COMPARE_COLUMNS.map(column => (
                    <td key={column} className="px-4 text-center">
                      <Verdict value={COMPARE_VERDICTS[column][index] ?? "no"} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        <p className="m-0 mt-10 max-w-[760px] text-[16px]/[1.5] text-site-dark-700">
          Why it holds: one runtime, one state model. Sessions, agents, approvals, and runs are core
          objects, so nothing has to be glued together.
        </p>
        <p className="m-0 mt-3 text-[13px] text-site-dark-450">
          Checked against the public documentation of{" "}
          {Object.entries(COMPARE_SOURCES).map(([name, href], index, all) => (
            <span key={name}>
              <a
                href={href}
                target="_blank"
                rel="noreferrer"
                className="underline decoration-site-dark-200 underline-offset-2 hover:text-[#0a0a0a]"
              >
                {name}
              </a>
              {index < all.length - 2 ? ", " : index === all.length - 2 ? " and " : ""}
            </span>
          ))}{" "}
          on {COMPARE_CHECKED_ON}. Partial means the docs cover part of the row.
        </p>
      </div>
    </section>
  );
}

const TAB = "h-11 rounded-[12px] px-4 text-[14.5px] after:hidden data-[active]:bg-site-light-100";

/** `releaseTag` pins the Go install line to the current release. */
export function Install({ releaseTag }: { releaseTag: string }) {
  const methods = installMethods(releaseTag);
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
          <div className="overflow-hidden rounded-[22px] border border-site-light-120 bg-[#101010] shadow-[0_0_120px_-40px_rgba(31,135,252,0.6)]">
            <Tabs defaultValue="Desktop" className="gap-0">
              <div className="border-b border-site-light-100 p-2">
                <TabsList className="flex-wrap gap-1 border-b-0 group-data-horizontal/tabs:h-auto">
                  {["Desktop", ...methods.map(method => method.id)].map(id => (
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
                <PrimaryCta icon={Download} href={HREF.download}>
                  Download for macOS
                </PrimaryCta>
                <TextLink href={HREF.download}>All platforms and checksums</TextLink>
              </TabsContent>
              {methods.map(method => (
                <TabsContent key={method.id} value={method.id} className="min-h-[132px] p-7">
                  <InstallCommand command={method.command} className="w-full justify-between" />
                </TabsContent>
              ))}
            </Tabs>
            <ol className="m-0 list-none border-t border-site-light-100 p-0">
              {INSTALL_STEPS.map((step, index) => (
                <li
                  key={step.command}
                  className={cn("flex gap-4 p-7", index > 0 && "border-t border-site-light-100")}
                >
                  <span
                    className={cn(
                      "grid size-8 shrink-0 place-items-center rounded-full font-mono text-[13px]",
                      index === 0 ? "bg-brand text-brand-ink" : "bg-site-light-100 text-fg"
                    )}
                  >
                    {index + 1}
                  </span>
                  <div className="min-w-0">
                    <p className="m-0 text-[17px] text-fg">{step.title}</p>
                    <p className="m-0 mt-2 font-mono text-[15px] break-words text-brand-strong">
                      {index === 0 ? <Typed text={step.command} /> : step.command}
                    </p>
                    {step.note ? <Muted className="mt-1">{step.note}</Muted> : null}
                  </div>
                </li>
              ))}
            </ol>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export function FinalCta() {
  return (
    <FinalScene>
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
              <PrimaryCta icon={Download} href={HREF.download}>
                Download CompozyOS
              </PrimaryCta>
              <SecondaryCta icon={Star} href={siteConfig.repoUrl}>
                Star on GitHub
              </SecondaryCta>
            </div>
            <InstallCommand command={INSTALL_COMMAND} className="mt-4" />
          </Reveal>
        </div>
      </div>
    </FinalScene>
  );
}
