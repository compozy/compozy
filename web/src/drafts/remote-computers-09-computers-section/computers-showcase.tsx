import { Button, Tabs, TabsList, TabsTrigger, cn, toast } from "@compozy/ui";
import {
  Check,
  Globe,
  Lock,
  Plus,
  Route,
  ServerIcon,
  ShieldCheck,
  Smartphone,
  Terminal,
} from "@compozy/ui/icons";
import { useState } from "react";

import { PlanUsageMeter } from "@/systems/account/components/plan-usage-meter";
import { ComputerRow } from "@/systems/computers/components/computer-row";
import {
  COMPUTERS_PRO,
  type ComputerFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  PHONE_FULL_SCOPES,
  PHONE_HOSTED_URL,
  PHONE_PEERS_MULTI,
  PHONE_TARGET_COMPUTER,
} from "@/systems/computers/mocks/remote-computers-fixtures-phone";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

import orderPlate from "../landing/assets/plate-order.jpg";
import { CONTAINER, HAIRLINE } from "../landing/layout";
import {
  H2,
  LandingEyebrow,
  Muted,
  Plate,
  ProductWindow,
  TextLink,
  type Icon,
} from "../landing/primitives";
import { PhoneApp } from "../remote-computers-07-phone/phone-app";
import { RunOnShot } from "./run-on-shot";

export type ShowcaseTab = "computers" | "run-on" | "phone";

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
    body: "Pick any host from your SSH config. Compozy verifies the checksum and starts the background service for you.",
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

const FEATURED_COMPUTERS: readonly ComputerFixture[] = [
  COMPUTERS_PRO[1]!, // studio (connected, LAN)
  COMPUTERS_PRO[2]!, // devbox (connected, SSH)
  COMPUTERS_PRO[0]!, // mini-build (needs_attention)
  COMPUTERS_PRO[3]!, // hetzner-eu (connecting)
];

function ComputersListShot() {
  const [computers, setComputers] = useState<ComputerFixture[]>([...FEATURED_COMPUTERS]);

  const handleToggle = (computer: ComputerFixture, nextEnabled: boolean) => {
    setComputers(current =>
      current.map(item =>
        item.id === computer.id
          ? { ...item, enabled: nextEnabled, status: nextEnabled ? "connected" : "disabled" }
          : item
      )
    );
  };

  return (
    <div className="flex h-full flex-col bg-canvas text-fg">
      <div className="flex h-11 shrink-0 items-center justify-between border-b border-line bg-rail px-4">
        <div className="flex items-center gap-3">
          <div aria-hidden className="flex items-center gap-1.5">
            <span className="size-2.5 rounded-full bg-line-strong" />
            <span className="size-2.5 rounded-full bg-line-strong" />
            <span className="size-2.5 rounded-full bg-line-strong" />
          </div>
          <span className="text-small-body font-medium text-fg">Settings</span>
          <span className="text-faint">/</span>
          <span className="text-small-body text-muted-foreground">Computers</span>
        </div>
        <Button
          size="xs"
          variant="secondary"
          type="button"
          onClick={() => toast("Opens Add computer dialog")}
        >
          <Plus aria-hidden className="size-3.5" />
          Add computer
        </Button>
      </div>

      <div className="flex-1 overflow-y-auto p-5 md:p-6">
        <div className="mx-auto max-w-[900px] space-y-4">
          <div className="flex flex-wrap items-baseline justify-between gap-3 border-b border-line pb-3">
            <div>
              <p className="m-0 text-detail-h1 font-medium text-fg">Computers</p>
              <p className="m-0 mt-0.5 text-small-body text-muted-foreground">
                3 connected · 1 connecting · Pro plan
              </p>
            </div>
            <PlanUsageMeter
              compact
              label="Hosted routes"
              limit={10}
              unitLabel="hosted routes"
              used={3}
            />
          </div>

          <SettingsGroup title={`Computers (${computers.length})`}>
            {computers.map(computer => (
              <ComputerRow
                computer={computer}
                key={computer.id}
                onOpenDetails={item => toast(`Inspecting ${item.name}`)}
                onRemove={item => toast(`Remove ${item.name}`)}
                onRename={item => toast(`Rename ${item.name}`)}
                onToggleEnabled={handleToggle}
              />
            ))}
          </SettingsGroup>
        </div>
      </div>
    </div>
  );
}

function PhoneShot() {
  return (
    <div className="grid h-full items-center gap-6 bg-canvas p-5 md:p-7 lg:grid-cols-[minmax(0,1fr)_390px]">
      <div className="flex flex-col justify-center gap-4 px-2 lg:px-5">
        <div className="inline-flex w-fit items-center gap-2 rounded-pill border border-line bg-sunken px-3 py-1 font-mono text-[12px] text-muted-foreground">
          <Lock aria-hidden className="size-3 text-success" />
          <span>{PHONE_HOSTED_URL}</span>
        </div>
        <h3 className="m-0 font-display text-[24px]/[1.15] font-normal tracking-[-0.02em] text-fg md:text-[30px]">
          Every session on your computer, in your pocket.
        </h3>
        <p className="m-0 text-[15px]/[1.55] text-muted-foreground">
          Turn on Hosted route for any machine on Pro. Sign in from your phone browser with Google
          or GitHub to read live transcripts, approve waiting commands, or open a remote terminal.
        </p>
        <ul className="m-0 list-none space-y-2.5 p-0 pt-1">
          {[
            "Installable as a home-screen web app on iOS and Android",
            "Device-scoped permissions and optional first-time approval",
            "Switch between computers with hosted routes from one menu",
          ].map(point => (
            <li key={point} className="flex items-start gap-2.5 text-[14.5px] text-fg/85">
              <Check aria-hidden className="mt-0.5 size-4 shrink-0 text-brand-strong" />
              <span>{point}</span>
            </li>
          ))}
        </ul>
      </div>

      <div className="mx-auto flex h-[500px] w-full max-w-[390px] flex-col overflow-hidden rounded-[24px] border border-[color:var(--light-150)] bg-canvas shadow-[0_24px_70px_-20px_rgba(0,0,0,0.9)]">
        <div className="flex h-9 shrink-0 items-center justify-center gap-1.5 border-b border-line bg-rail px-3">
          <Lock aria-hidden className="size-3 text-subtle" />
          <span className="truncate font-mono text-[11.5px] text-muted-foreground">
            {PHONE_HOSTED_URL}
          </span>
        </div>
        <div className="flex min-h-0 flex-1 flex-col overflow-hidden">
          <PhoneApp
            computer={PHONE_TARGET_COMPUTER}
            peers={PHONE_PEERS_MULTI}
            scopes={PHONE_FULL_SCOPES}
            showInstallHint={false}
          />
        </div>
      </div>
    </div>
  );
}

export function ComputersLandingSection({
  initialTab = "computers",
  openRunOnPicker = false,
}: {
  initialTab?: ShowcaseTab;
  openRunOnPicker?: boolean;
}) {
  const [active, setActive] = useState<ShowcaseTab>(initialTab);

  return (
    <section
      id="computers"
      className={cn(
        "relative scroll-mt-16 overflow-x-clip border-t bg-[#070707] py-12 md:py-16",
        HAIRLINE
      )}
    >
      {/* Restrained plate tucked behind the top-right corner only, never crossing copy legibility. */}
      <Plate
        src={orderPlate}
        className="top-0 right-[-8%] h-[280px] w-[48%] opacity-30 [mask-image:linear-gradient(90deg,transparent,black_55%),linear-gradient(180deg,black_35%,transparent)] [mask-composite:intersect]"
      />

      <div className={cn(CONTAINER, "relative")}>
        <Tabs value={active} onValueChange={value => setActive(value as ShowcaseTab)} className="gap-0">
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
              <TabsList className="h-auto w-fit flex-wrap gap-1 rounded-[14px] border border-[color:var(--light-120)] bg-[#111111] p-1 group-data-horizontal/tabs:h-auto">
                {SHOWCASE_TABS.map(tab => {
                  const isSelected = tab.id === active;
                  return (
                    <TabsTrigger
                      key={tab.id}
                      value={tab.id}
                      className={cn(
                        "h-9 gap-2 rounded-[10px] px-3.5 text-[13.5px] after:hidden",
                        isSelected && "bg-[color:var(--light-100)] text-fg"
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
            {active === "run-on" ? <RunOnShot pickerDefaultOpen={openRunOnPicker} /> : null}
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
            <TextLink>Read the Computers docs</TextLink>
            <TextLink>See pricing</TextLink>
          </div>
        </div>
      </div>
    </section>
  );
}
