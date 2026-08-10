import { allPosts } from "@/lib/blog";
import { loadChangelogReleases } from "@/lib/changelog/github-client";
import { DOCS_GROUP_BY_FOLDER, DOCS_ROOT_GROUP, docsGroupForUrl } from "@/lib/docs-navigation";
import { absoluteUrl } from "@/lib/site-config";
import { docsSource } from "@/lib/source";

export const revalidate = 300;

function section(title: string, lines: string[]): string {
  if (!lines.length) return "";
  return `## ${title}\n\n${lines.join("\n")}\n`;
}

function docLine(title: string, url: string, description?: string): string {
  const desc = description ? `: ${description}` : "";
  return `- [${title}](${absoluteUrl(url)})${desc}`;
}

function groupOrder(): string[] {
  return Array.from(new Set([DOCS_ROOT_GROUP, ...DOCS_GROUP_BY_FOLDER.values()]));
}

export async function GET() {
  const pages = docsSource.getPages();
  const posts = allPosts();
  const changelog = await loadChangelogReleases();
  const currentRelease = changelog.status === "ready" ? changelog.releases[0] : undefined;

  const linesByGroup = new Map<string, string[]>();
  for (const page of pages) {
    const group = docsGroupForUrl(page.url);
    const lines = linesByGroup.get(group) ?? [];
    lines.push(docLine(page.data.title, page.url, page.data.description));
    linesByGroup.set(group, lines);
  }

  const blogLines = posts.map(post => docLine(post.title, post.permalink, post.description));
  const releaseLines = [
    ...(currentRelease
      ? [
          docLine(
            `Current release: ${currentRelease.version}`,
            `/changelog/${encodeURIComponent(currentRelease.version)}`,
            `${currentRelease.channel} — ${currentRelease.summary}`
          ),
        ]
      : []),
    docLine(
      "Migrate from CompozyOS v0.2.15 to CompozyOS v0.3",
      "/docs/migration",
      "Hard-cut command, configuration, storage, and extension migration guidance."
    ),
  ];

  const body = [
    "# CompozyOS Documentation",
    "",
    "> CompozyOS runs agent work, state, memory, permissions, coordination, and extensibility in one local-first runtime.",
    "",
    section("Current release and migration", releaseLines),
    ...groupOrder().map(group => section(group, linesByGroup.get(group) ?? [])),
    section("Blog", blogLines),
  ]
    .filter(Boolean)
    .join("\n");

  return new Response(body, {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
