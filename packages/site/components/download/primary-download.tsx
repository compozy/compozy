"use client";

import { buttonVariants, cn } from "@compozy/ui";
import { Download } from "lucide-react";
import {
  DESKTOP_TARGETS,
  primaryTargetFor,
  type DesktopRelease,
  type DesktopTargetId,
  type VisitorPlatform,
} from "@/lib/release/desktop-downloads";
import { useVisitorPlatform } from "./use-visitor-platform";

interface Offer {
  href: string;
  label: string;
  detail: string;
}

function targetOffer(release: DesktopRelease, id: DesktopTargetId): Offer | null {
  const target = DESKTOP_TARGETS.find(item => item.id === id);
  if (!target) return null;
  const label = `Download for ${target.os}`;
  if (release.status === "fallback") {
    return {
      href: release.releaseUrl,
      label,
      detail: `${target.arch} · ${target.format} · latest release on GitHub`,
    };
  }
  const download = release.downloads.find(item => item.target.id === id);
  if (!download) return null;
  return {
    href: download.url,
    label,
    detail: `${target.arch} · ${target.format} · ${release.tag}`,
  };
}

/** The one alternative a visitor on this platform is most likely to need instead. */
function alternateFor(platform: VisitorPlatform): { id: DesktopTargetId; prompt: string } | null {
  switch (platform) {
    case "mac-arm64":
    case "mac":
    case "unknown":
      return { id: "mac-x64", prompt: "On an Intel Mac?" };
    case "mac-x64":
      return { id: "mac-arm64", prompt: "On Apple silicon?" };
    case "linux":
      return { id: "linux-deb", prompt: "Prefer a .deb package?" };
    case "windows":
    case "mobile":
      return null;
  }
}

const ALTERNATE_LABEL: Record<DesktopTargetId, string> = {
  "mac-arm64": "Download the Apple silicon build",
  "mac-x64": "Download the Intel build",
  "linux-appimage": "Download the AppImage",
  "linux-deb": "Download the .deb",
};

const UNSUPPORTED_COPY: Record<"windows" | "mobile", { label: string; detail: string }> = {
  windows: {
    label: "No desktop app for Windows yet",
    detail: "The desktop app ships for macOS and Linux. See the Windows note below.",
  },
  mobile: {
    label: "Desktop app for macOS and Linux",
    detail: "Open this page on your computer to get the right package.",
  },
};

export function PrimaryDownload({ release }: { release: DesktopRelease }) {
  const platform = useVisitorPlatform();
  const targetId = primaryTargetFor(platform);
  const offer = targetId ? targetOffer(release, targetId) : null;
  const alternate = alternateFor(platform);
  const alternateOffer = alternate ? targetOffer(release, alternate.id) : null;

  // Every state renders the same three rows (action, detail, alternate) at fixed
  // heights, so the post-hydration platform swap never moves the layout.
  return (
    <div className="flex flex-col items-start gap-3" data-platform={platform}>
      {offer ? (
        <a
          href={offer.href}
          className={cn(buttonVariants({ variant: "default", size: "cta-lg" }), "min-w-60")}
        >
          <Download aria-hidden data-icon="inline-start" />
          {offer.label}
        </a>
      ) : (
        <a
          href="#all-downloads"
          className={cn(buttonVariants({ variant: "outline", size: "cta-lg" }), "min-w-60")}
        >
          {platform === "windows" || platform === "mobile"
            ? UNSUPPORTED_COPY[platform].label
            : "See all downloads"}
        </a>
      )}

      <p className={cn("min-h-5 text-subtle", offer ? "font-mono text-meta" : "text-sm")}>
        {offer
          ? offer.detail
          : platform === "windows" || platform === "mobile"
            ? UNSUPPORTED_COPY[platform].detail
            : null}
      </p>

      <p className="min-h-5 text-sm text-muted-foreground">
        {alternate && alternateOffer ? (
          <>
            {alternate.prompt}{" "}
            <a
              href={alternateOffer.href}
              className="text-fg underline decoration-line-strong underline-offset-4 transition-colors hover:text-brand-strong hover:decoration-brand"
            >
              {ALTERNATE_LABEL[alternate.id]}
            </a>
          </>
        ) : (
          <a
            href="#all-downloads"
            className="text-fg underline decoration-line-strong underline-offset-4 transition-colors hover:text-brand-strong hover:decoration-brand"
          >
            Other platforms and packages
          </a>
        )}
      </p>
    </div>
  );
}
