import type { Metadata } from "next";
import { CommandInstall, DownloadCatalog, DownloadHero } from "@/components/download";
import { getDesktopRelease } from "@/lib/release/desktop-downloads";
import { createPageMetadata } from "@/lib/site-config";

export const revalidate = 300;

export const metadata: Metadata = createPageMetadata({
  title: "Download CompozyOS",
  description:
    "Download the CompozyOS desktop app for macOS (Apple silicon and Intel) and Linux, or install the compozy command from your terminal.",
  path: "/download",
  keywords: ["CompozyOS download", "CompozyOS desktop app", "macOS", "Linux", "AppImage", "DMG"],
});

export default async function DownloadPage() {
  const release = await getDesktopRelease();
  return (
    <>
      <DownloadHero release={release} />
      <DownloadCatalog release={release} />
      <CommandInstall />
    </>
  );
}
