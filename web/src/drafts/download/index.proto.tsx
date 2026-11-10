import "../landing/landing.css";

import { definePrototype, type PrototypeProps } from "@compozy/proto";
import { LazyMotion, domMax } from "motion/react";
import { useState } from "react";

import { SiteFooter } from "../landing/sections-close";
import { SiteHeader } from "../landing/sections-open";
import { DownloadHero, type HeroMode } from "./hero";
import type { MacArch, Platform } from "./release";
import { AllDownloads, FirstRun, FromTheTerminal } from "./sections";

export const meta = definePrototype({
  title: "Download page redesign",
  spec: "landing-redesign",
  surface: "compozy.com/download",
  states: [
    { id: "macos", label: "macOS, Apple silicon" },
    { id: "intel", label: "macOS, Intel" },
    { id: "linux", label: "Linux" },
    { id: "windows", label: "Windows" },
    { id: "mobile", label: "Phone or tablet" },
    { id: "unavailable", label: "GitHub unavailable" },
  ],
});

interface Visitor {
  mode: HeroMode;
  platform: Platform;
  arch: MacArch;
}

const START: Record<string, Visitor> = {
  macos: { mode: "ready", platform: "macos", arch: "arm64" },
  intel: { mode: "ready", platform: "macos", arch: "x64" },
  linux: { mode: "ready", platform: "linux", arch: "arm64" },
  windows: { mode: "ready", platform: "windows", arch: "arm64" },
  mobile: { mode: "mobile", platform: "macos", arch: "arm64" },
  unavailable: { mode: "unavailable", platform: "macos", arch: "arm64" },
};

function DownloadPage({ initialVisitor }: { initialVisitor: Visitor }) {
  const [platform, setPlatform] = useState(initialVisitor.platform);
  const [arch, setArch] = useState(initialVisitor.arch);
  const { mode } = initialVisitor;
  return (
    <>
      <DownloadHero
        mode={mode}
        platform={platform}
        arch={arch}
        onPlatform={setPlatform}
        onArch={setArch}
      />
      <AllDownloads unavailable={mode === "unavailable"} />
      <FromTheTerminal />
      <FirstRun platform={platform} />
    </>
  );
}

export default function Prototype({ state }: PrototypeProps) {
  return (
    <LazyMotion features={domMax}>
      <div id="landing" className="dark bg-[#0a0a0a] font-sans text-fg antialiased">
        <SiteHeader />
        <main>
          <DownloadPage key={state} initialVisitor={START[state] ?? START.macos} />
        </main>
        <SiteFooter />
      </div>
    </LazyMotion>
  );
}
