"use client";

/** The download page: the detected platform starts the switch; the visitor's pick overrides it. */
import { useState } from "react";

import type { DesktopRelease } from "@/lib/release/desktop-downloads";
import { choiceFor, heroModeFor, type Choice } from "@/lib/release/download-offer";
import { DownloadHero } from "./download-hero";
import { AllDownloads, FirstRun, FromTheTerminal } from "./sections";
import { useVisitorPlatform } from "./use-visitor-platform";

export function DownloadPage({
  release,
  releaseDate,
}: {
  release: DesktopRelease;
  releaseDate: string | null;
}) {
  const visitor = useVisitorPlatform();
  const [picked, setPicked] = useState<Choice | null>(null);
  const choice = picked ?? choiceFor(visitor);
  const mode = heroModeFor(release, visitor);
  return (
    <div data-platform={visitor}>
      <DownloadHero
        mode={mode}
        release={release}
        choice={choice}
        onPlatform={platform => setPicked({ ...choice, platform })}
        onArch={arch => setPicked({ ...choice, arch })}
      />
      <AllDownloads release={release} releaseDate={releaseDate} />
      <FromTheTerminal tag={release.status === "ready" ? release.tag : null} />
      <FirstRun platform={choice.platform} />
    </div>
  );
}
