import { loadChangelogReleases } from "@/lib/changelog/github-client";
import { CHANGELOG_REVALIDATE_SECONDS } from "@/lib/changelog/types";
import { absoluteUrl, siteConfig } from "@/lib/site-config";

export const revalidate = 300;

function escapeXml(input: string): string {
  return input
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;")
    .replace(/'/g, "&apos;");
}

export async function GET() {
  const changelog = await loadChangelogReleases();
  if (changelog.status === "unavailable") {
    return new Response("GitHub changelog is temporarily unavailable.\n", {
      status: 503,
      headers: { "content-type": "text/plain; charset=utf-8", "retry-after": "300" },
    });
  }
  const items = changelog.releases
    .map(release => {
      const link = absoluteUrl(`/changelog/${encodeURIComponent(release.version)}`);
      return [
        "<item>",
        `<title>${escapeXml(release.version)}</title>`,
        `<link>${link}</link>`,
        `<guid isPermaLink="true">${link}</guid>`,
        `<pubDate>${new Date(release.publishedAt).toUTCString()}</pubDate>`,
        `<description>${escapeXml(release.body)}</description>`,
        `<category>${escapeXml(release.channel)}</category>`,
        "</item>",
      ].join("");
    })
    .join("");
  const xml = `<?xml version="1.0" encoding="UTF-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom">
  <channel>
    <title>${escapeXml(siteConfig.name)} changelog</title>
    <link>${absoluteUrl("/changelog")}</link>
    <description>Published CompozyOS release notes.</description>
    <language>en-us</language>
    <atom:link href="${absoluteUrl("/changelog/feed.xml")}" rel="self" type="application/rss+xml" />
    ${items}
  </channel>
</rss>`;
  return new Response(xml, {
    headers: {
      "content-type": "application/rss+xml; charset=utf-8",
      "cache-control": `s-maxage=${CHANGELOG_REVALIDATE_SECONDS}, stale-while-revalidate=86400`,
    },
  });
}
