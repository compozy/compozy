import { Tabs, TabsContent, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { Check, Download } from "@compozy/ui/icons";

import { COMPARE_COLUMNS, INSTALL_COMMAND } from "../landing/content";
import { CONTAINER } from "../landing/layout";
import {
  H2,
  InstallCommand,
  LandingEyebrow,
  Lead,
  Muted,
  PrimaryCta,
  TextLink,
} from "../landing/primitives";

const UPDATED_COMPARE_ROWS: ReadonlyArray<{
  readonly label: string;
  readonly compozy: "check" | string;
  readonly competitors: Readonly<Record<string, string>>;
}> = [
  {
    label: "Runs the agents you already use",
    compozy: "check",
    competitors: {
      Synara: "Yes",
      "T3 Code": "Yes",
      Hermes: "Own agent",
      OpenClaw: "Own agent",
      "DIY stack": "By hand",
    },
  },
  {
    label: "Always-on agents that check in on their own",
    compozy: "check",
    competitors: {
      Synara: "No",
      "T3 Code": "No",
      Hermes: "Limited",
      OpenClaw: "Cron only",
      "DIY stack": "Scripts",
    },
  },
  {
    label: "One board by who owns the next step",
    compozy: "check",
    competitors: {
      Synara: "List",
      "T3 Code": "List",
      Hermes: "No",
      OpenClaw: "No",
      "DIY stack": "No",
    },
  },
  {
    label: "Loops, schedules, and triggers",
    compozy: "check",
    competitors: {
      Synara: "No",
      "T3 Code": "No",
      Hermes: "Basic",
      OpenClaw: "Basic",
      "DIY stack": "Glued",
    },
  },
  {
    label: "Remote computers and phone access",
    compozy: "Free + Pro",
    competitors: {
      Synara: "Paid plan",
      "T3 Code": "SSH",
      Hermes: "No",
      OpenClaw: "Self-hosted",
      "DIY stack": "Tailscale + SSH",
    },
  },
  {
    label: "Runs on your computers. Open source (MIT).",
    compozy: "check",
    competitors: {
      Synara: "Closed",
      "T3 Code": "MIT",
      Hermes: "Open",
      OpenClaw: "MIT",
      "DIY stack": "Mixed",
    },
  },
];

/** Updated comparison section (ADR-014 §3): corrected MIT row and honest Free + Pro listing alongside Synara. */
export function UpdatedComparison() {
  return (
    <section
      id="comparison"
      className="relative scroll-mt-20 bg-[#f4f4f1] py-14 text-[#0a0a0a] md:py-20"
    >
      <div className={CONTAINER}>
        <div className="max-w-[820px]">
          <p className="m-0 mb-4 inline-flex rounded-pill border border-[color:var(--dark-150)] px-3.5 py-1 font-mono text-[12.5px]">
            Side by side
          </p>
          <h2 className="m-0 font-display text-[36px]/[1.06] font-normal tracking-[-0.03em] md:text-[54px]/[1.04]">
            Every piece, side by side.
          </h2>
          <p className="m-0 mt-3 text-[16.5px]/[1.5] text-[color:var(--dark-600)] md:text-[18px]/[1.45]">
            Synara, T3 Code, Hermes, and OpenClaw each prove demand for part of the system.
            CompozyOS ships those parts in one product.
          </p>
        </div>

        <div className="mt-8 overflow-x-auto md:mt-10">
          <table className="w-full min-w-[860px] border-collapse text-left text-[15.5px]">
            <thead>
              <tr>
                <th className="w-[36%] pb-4 font-normal" />
                <th className="rounded-t-[14px] bg-[#2448e6] px-4 py-4 text-center font-medium text-white">
                  CompozyOS
                </th>
                {COMPARE_COLUMNS.map(column => (
                  <th
                    key={column}
                    className="px-4 pb-4 text-center font-normal text-[color:var(--dark-550)]"
                  >
                    {column}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {UPDATED_COMPARE_ROWS.map((row, index) => (
                <tr
                  key={row.label}
                  className="border-t border-[color:var(--dark-100)] transition-colors hover:bg-[color:var(--dark-030)]"
                >
                  <td className="py-4 pr-6 text-[#0a0a0a]">{row.label}</td>
                  <td
                    className={cn(
                      "bg-[#2448e6] px-4 text-center text-white",
                      index === UPDATED_COMPARE_ROWS.length - 1 && "rounded-b-[14px]"
                    )}
                  >
                    {row.compozy === "check" ? (
                      <Check aria-label="Yes" className="mx-auto size-5 text-white" />
                    ) : (
                      <span className="font-mono text-[13px] font-medium text-white">
                        {row.compozy}
                      </span>
                    )}
                  </td>
                  {COMPARE_COLUMNS.map(column => (
                    <td
                      key={column}
                      className="px-4 text-center font-mono text-[12.5px] text-[color:var(--dark-550)]"
                    >
                      {row.competitors[column]}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        <p className="m-0 mt-10 max-w-[760px] text-[16px]/[1.5] text-[color:var(--dark-700)]">
          Why it holds: one runtime, one state model. Sessions, agents, approvals, and remote
          computers are core objects, so nothing has to be glued together.
        </p>
        <p className="m-0 mt-3 text-[13px] text-[color:var(--dark-450)]">
          Checked against each product's public documentation on Nov 23, 2026.
        </p>
      </div>
    </section>
  );
}

const OFFICIAL_INSTALL_METHODS = [
  { id: "Installer", command: INSTALL_COMMAND },
  { id: "Homebrew", command: "brew install compozy/compozy/compozy" },
  { id: "npm", command: "npm install -g @compozy/cli" },
] as const;

const INSTALL_STEPS: ReadonlyArray<{
  readonly title: string;
  readonly command: string;
  readonly note?: string;
}> = [
  {
    title: "Pick a provider",
    command: "compozy install",
    note: "The verified installer opens this for you.",
  },
  {
    title: "Sign in",
    command: "compozy account login",
    note: "Sign in with Google or GitHub on first run. Later use works offline.",
  },
  {
    title: "Open a session",
    command: "compozy session new --agent general --name first-run",
    note: "From the repository you want to work on, or pick a remote computer with Run on.",
  },
];

const TAB =
  "h-11 rounded-[12px] px-4 text-[14.5px] after:hidden data-[active]:bg-[color:var(--light-100)]";

/** Updated install section (ADR-014 §3, Q6 = b): shipped install → Sign in → open a session, no `go install`. */
export function UpdatedInstall() {
  return (
    <section id="install" className="relative scroll-mt-20 py-16 md:py-24">
      <div className={cn(CONTAINER, "grid items-start gap-12 lg:grid-cols-[1fr_1.15fr] lg:gap-16")}>
        <div>
          <LandingEyebrow>Getting started</LandingEyebrow>
          <H2 size="sm">Install CompozyOS. Start one session.</H2>
          <Lead>
            Download the desktop app, or install the runtime with the verified installer, Homebrew,
            or npm. Sign in once with Google or GitHub; local work continues offline after that.
          </Lead>
        </div>

        <div className="overflow-hidden rounded-[22px] border border-[color:var(--light-120)] bg-[#101010] shadow-[0_0_120px_-40px_rgba(31,135,252,0.6)]">
          <Tabs defaultValue="Desktop" className="gap-0">
            <div className="border-b border-[color:var(--light-100)] p-2">
              <TabsList className="flex-wrap gap-1 border-b-0 group-data-horizontal/tabs:h-auto">
                {["Desktop", ...OFFICIAL_INSTALL_METHODS.map(method => method.id)].map(id => (
                  <TabsTrigger key={id} value={id} className={TAB}>
                    {id}
                  </TabsTrigger>
                ))}
              </TabsList>
            </div>
            <TabsContent
              value="Desktop"
              className="flex min-h-[108px] flex-wrap items-center gap-6 p-6"
            >
              <PrimaryCta icon={Download}>Download for macOS</PrimaryCta>
              <TextLink>All platforms and checksums</TextLink>
            </TabsContent>
            {OFFICIAL_INSTALL_METHODS.map(method => (
              <TabsContent key={method.id} value={method.id} className="min-h-[108px] p-6">
                <InstallCommand command={method.command} className="w-full justify-between" />
              </TabsContent>
            ))}
          </Tabs>

          <ol className="m-0 list-none border-t border-[color:var(--light-100)] p-0">
            {INSTALL_STEPS.map((step, index) => (
              <li
                key={step.command}
                className={cn(
                  "flex gap-4 p-5 sm:px-6",
                  index > 0 && "border-t border-[color:var(--light-100)]"
                )}
              >
                <span
                  className={cn(
                    "grid size-8 shrink-0 place-items-center rounded-full font-mono text-[13px]",
                    index === 0 ? "bg-brand text-brand-ink" : "bg-[color:var(--light-100)] text-fg"
                  )}
                >
                  {index + 1}
                </span>
                <div className="min-w-0">
                  <p className="m-0 text-[16.5px] text-fg">{step.title}</p>
                  <p className="m-0 mt-1.5 font-mono text-[14.5px] break-words text-brand-strong">
                    {step.command}
                  </p>
                  {step.note ? <Muted className="mt-1 text-[14.5px]">{step.note}</Muted> : null}
                </div>
              </li>
            ))}
          </ol>
        </div>
      </div>
    </section>
  );
}
