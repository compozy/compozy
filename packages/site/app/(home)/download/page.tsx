import type { Metadata } from "next";
import { DownloadPage } from "@/components/download/download-page";
import { formatDate } from "@/lib/changelog/timeline";
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

export default async function Page() {
  const release = await getDesktopRelease();
  const releaseDate = release.status === "ready" ? formatDate(release.publishedAt) : null;
  return <DownloadPage release={release} releaseDate={releaseDate} />;
}
