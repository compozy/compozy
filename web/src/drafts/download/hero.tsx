/** One clear action for this machine, with the app itself right underneath. */
import { Button, Logo, Tabs, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { Check, Copy, Download, ExternalLink } from "@compozy/ui/icons";
import { useState, type ComponentType, type SVGProps } from "react";

import horizon from "../landing/assets/plate-watch.jpg";
import { CONTAINER } from "../landing/layout";
import { H1, Lead, Plate, PrimaryCta, Small } from "../landing/primitives";
import { AppleMark, LinuxMark, WindowsMark } from "./os-logos";
import {
  DESKTOP,
  LATEST_URL,
  RELEASE,
  artifactUrl,
  megabytes,
  offerFor,
  type MacArch,
  type Platform,
} from "./release";

const PLATFORMS: ReadonlyArray<
  readonly [Platform, string, ComponentType<SVGProps<SVGSVGElement>>]
> = [
  ["macos", "macOS", AppleMark],
  ["linux", "Linux", LinuxMark],
  ["windows", "Windows", WindowsMark],
];

const LINK =
  "cursor-pointer bg-transparent p-0 text-[14px] text-fg underline decoration-fg/30 underline-offset-4 transition-colors hover:decoration-fg";

function Alternate({
  platform,
  arch,
  onArch,
}: {
  platform: Platform;
  arch: MacArch;
  onArch: (arch: MacArch) => void;
}) {
  if (platform === "macos") {
    return (
      <Small className="text-[14px]">
        {arch === "arm64" ? "On an Intel Mac? " : "On Apple silicon? "}
        <button
          type="button"
          className={LINK}
          onClick={() => onArch(arch === "arm64" ? "x64" : "arm64")}
        >
          {arch === "arm64" ? "Get the Intel build" : "Get the Apple silicon build"}
        </button>
      </Small>
    );
  }
  if (platform === "linux") {
    return (
      <Small className="text-[14px]">
        Prefer a package?{" "}
        <a className={LINK} href={artifactUrl(DESKTOP.deb)}>
          Get the deb
        </a>
      </Small>
    );
  }
  return (
    <Small className="text-[14px]">
      The desktop app is for macOS and Linux today. On Windows, you get the compozy command.
    </Small>
  );
}

function Picker({
  platform,
  onPlatform,
}: {
  platform: Platform;
  onPlatform: (platform: Platform) => void;
}) {
  return (
    <Tabs value={platform} onValueChange={value => onPlatform(value as Platform)} className="gap-0">
      <TabsList className="h-auto gap-1 rounded-[14px] border border-[color:var(--light-100)] bg-[color:var(--dark-400)] p-1 backdrop-blur-md">
        {PLATFORMS.map(([id, label, Mark]) => (
          <TabsTrigger key={id} value={id} className="h-9 gap-2 rounded-[10px] px-3.5 text-[14px]">
            <span className="inline-flex items-center gap-2">
              <Mark className="size-3.5" />
              {label}
            </span>
          </TabsTrigger>
        ))}
      </TabsList>
    </Tabs>
  );
}

function CopyLink() {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      size="cta-lg"
      variant="outline"
      className="h-[52px] px-7 text-[15.5px]"
      onClick={() => {
        void navigator.clipboard?.writeText("https://compozy.com/download");
        setCopied(true);
      }}
    >
      {copied ? <Check /> : <Copy />}
      {copied ? "Link copied" : "Copy the download link"}
    </Button>
  );
}

export type HeroMode = "ready" | "mobile" | "unavailable";

function Actions({
  mode,
  platform,
  arch,
  onPlatform,
  onArch,
}: {
  mode: HeroMode;
  platform: Platform;
  arch: MacArch;
  onPlatform: (platform: Platform) => void;
  onArch: (arch: MacArch) => void;
}) {
  if (mode === "mobile") {
    return (
      <div className="mt-10 flex flex-col items-center gap-4">
        <CopyLink />
        <Small className="text-[14px]">
          CompozyOS runs on your computer. Open this page there to download it.
        </Small>
      </div>
    );
  }
  if (mode === "unavailable") {
    return (
      <div className="mt-10 flex flex-col items-center gap-4">
        <a href={LATEST_URL}>
          <PrimaryCta icon={ExternalLink}>Open the latest release</PrimaryCta>
        </a>
        <Small className="text-[14px]">Every package is listed on GitHub.</Small>
      </div>
    );
  }
  const offer = offerFor(platform, arch);
  return (
    <div className="mt-9 flex flex-col items-center gap-5">
      <Picker platform={platform} onPlatform={onPlatform} />
      <a href={artifactUrl(offer.item)}>
        <PrimaryCta icon={Download}>{offer.cta}</PrimaryCta>
      </a>
      <p className="m-0 font-mono text-[13px] text-muted-foreground">
        {offer.item.name} · {offer.item.detail} · {megabytes(offer.item.bytes)} · {RELEASE.version}
      </p>
      <Alternate platform={platform} arch={arch} onArch={onArch} />
    </div>
  );
}

export function DownloadHero(props: {
  mode: HeroMode;
  platform: Platform;
  arch: MacArch;
  onPlatform: (platform: Platform) => void;
  onArch: (arch: MacArch) => void;
}) {
  return (
    <section className="relative overflow-x-clip pb-40 md:pb-44">
      <Plate
        src={horizon}
        className="inset-x-0 bottom-0 h-[300px] w-full object-[50%_34%] opacity-70 [mask-image:linear-gradient(180deg,transparent,black_30%,black_55%,transparent)]"
      />
      <div
        className={cn(
          CONTAINER,
          "relative z-10 flex flex-col items-center pt-32 text-center md:pt-40"
        )}
      >
        <Logo variant="symbol" className="size-[64px]" />
        <div className="mt-6 [&_h1]:md:text-[72px] [&_h1]:md:leading-[1.02]">
          <H1>Download CompozyOS</H1>
        </div>
        <Lead className="mt-5 max-w-[600px] md:text-[19px]">
          The desktop app for macOS and Linux, with the runtime included. The command line also runs
          on Windows.
        </Lead>
        <Actions {...props} />
      </div>
    </section>
  );
}
