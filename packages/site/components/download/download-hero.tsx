"use client";

/** One clear action for this machine, with the app itself right underneath. */
import { Button, Logo, Tabs, TabsList, TabsTrigger, cn } from "@compozy/ui";
import { Check, Copy, Download, ExternalLink, TriangleAlert } from "@compozy/ui/icons";
import type { ComponentType, SVGProps } from "react";

import { ASSET } from "@/components/landing/assets";
import { useCopyFeedback } from "@/components/site/hooks/use-copy-feedback";
import { CONTAINER } from "@/components/site/layout";
import { H1, Lead, Plate, PrimaryCta, Small } from "@/components/site/primitives";
import type { DesktopRelease } from "@/lib/release/desktop-downloads";
import {
  linuxDebItem,
  megabytes,
  offerFor,
  type Choice,
  type HeroMode,
  type MacArch,
  type Platform,
} from "@/lib/release/download-offer";
import { AppleMark, LinuxMark, WindowsMark } from "./os-logos";

const PLATFORMS: ReadonlyArray<
  readonly [Platform, string, ComponentType<SVGProps<SVGSVGElement>>]
> = [
  ["macos", "macOS", AppleMark],
  ["linux", "Linux", LinuxMark],
  ["windows", "Windows", WindowsMark],
];

const LINK =
  "cursor-pointer bg-transparent p-0 text-[14px] text-fg underline decoration-fg/30 underline-offset-4 transition-colors hover:decoration-fg";

const DOWNLOAD_PAGE_URL = "https://compozy.com/download";

function Alternate({
  release,
  choice,
  onArch,
}: {
  release: DesktopRelease;
  choice: Choice;
  onArch: (arch: MacArch) => void;
}) {
  const { platform, arch } = choice;
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
        <a className={LINK} href={linuxDebItem(release).href}>
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
      <TabsList className="h-auto gap-1 rounded-[14px] border border-site-light-100 bg-site-dark-400 p-1 backdrop-blur-md">
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

const COPY_LINK = {
  idle: { label: "Copy the download link", Glyph: Copy },
  copied: { label: "Link copied", Glyph: Check },
  failed: { label: "Couldn't copy the link", Glyph: TriangleAlert },
} as const;

function CopyLink() {
  const { state, copy } = useCopyFeedback(DOWNLOAD_PAGE_URL);
  const { label, Glyph } = COPY_LINK[state];
  return (
    <Button
      size="cta-lg"
      variant="outline"
      aria-live="polite"
      data-copy-state={state}
      className="h-[52px] px-7 text-[15.5px] data-[copy-state=failed]:text-destructive"
      onClick={() => void copy()}
    >
      <Glyph />
      {label}
    </Button>
  );
}

interface HeroProps {
  mode: HeroMode;
  release: DesktopRelease;
  choice: Choice;
  onPlatform: (platform: Platform) => void;
  onArch: (arch: MacArch) => void;
}

function Actions({ mode, release, choice, onPlatform, onArch }: HeroProps) {
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
  if (mode === "unavailable" || release.status !== "ready") {
    return (
      <div className="mt-10 flex flex-col items-center gap-4">
        <PrimaryCta icon={ExternalLink} href={release.releaseUrl}>
          Open the latest release
        </PrimaryCta>
        <Small className="text-[14px]">Every package is listed on GitHub.</Small>
      </div>
    );
  }
  const offer = offerFor(release, choice);
  const facts = [
    offer.item.name,
    offer.item.detail,
    offer.item.size === null ? null : megabytes(offer.item.size),
    release.tag,
  ].filter(Boolean);
  return (
    <div className="mt-9 flex flex-col items-center gap-5">
      <Picker platform={choice.platform} onPlatform={onPlatform} />
      {/* A listed file downloads in place; without one the button opens the release on GitHub. */}
      <PrimaryCta icon={Download} href={offer.item.href} download={offer.item.fileName !== null}>
        {offer.cta}
      </PrimaryCta>
      <p className="m-0 font-mono text-[13px] text-muted-foreground">{facts.join(" · ")}</p>
      <Alternate release={release} choice={choice} onArch={onArch} />
    </div>
  );
}

export function DownloadHero(props: HeroProps) {
  return (
    <section className="relative overflow-x-clip pb-40 md:pb-44">
      <Plate
        src={ASSET.plateWatch}
        load="eager"
        className="inset-x-0 bottom-0 h-[300px] w-full opacity-70 [mask-image:linear-gradient(180deg,transparent,black_30%,black_55%,transparent)]"
        imageClassName="object-[50%_34%]"
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
