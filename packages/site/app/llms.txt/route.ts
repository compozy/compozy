import { allPosts, allReleases } from "@/lib/blog";
import { absoluteUrl } from "@/lib/site-config";
import { protocolDocs, runtimeDocs } from "@/lib/source";

export const dynamic = "force-static";
export const revalidate = false;

function section(title: string, lines: string[]): string {
  if (!lines.length) return "";
  return `## ${title}\n\n${lines.join("\n")}\n`;
}

function docLine(title: string, url: string, description?: string): string {
  const desc = description ? `: ${description}` : "";
  return `- [${title}](${absoluteUrl(url)})${desc}`;
}

export function GET() {
  const runtimePages = runtimeDocs.getPages();
  const protocolPages = protocolDocs.getPages();
  const posts = allPosts();
  const currentRelease = allReleases()[0];

  const runtimeLines = runtimePages.map(page =>
    docLine(page.data.title, page.url, page.data.description)
  );
  const protocolLines = protocolPages.map(page =>
    docLine(page.data.title, page.url, page.data.description)
  );
  const blogLines = posts.map(post => docLine(post.title, post.permalink, post.description));
  const releaseLines = [
    ...(currentRelease
      ? [
          docLine(
            `Current release: ${currentRelease.version}`,
            `/changelog#${currentRelease.version}`,
            `${currentRelease.status} — ${currentRelease.summary}`
          ),
        ]
      : []),
    docLine(
      "Migrate from Compozy v0.2.15 to CompozyOS v0.3",
      "/runtime/migration",
      "Hard-cut command, configuration, storage, and extension migration guidance."
    ),
  ];

  const body = [
    "# CompozyOS Documentation",
    "",
    "> CompozyOS runs agent work, state, memory, permissions, coordination, and extensibility in one local-first runtime.",
    "",
    section("Current release and migration", releaseLines),
    section("Runtime", runtimeLines),
    section("Compozy Network", protocolLines),
    section("Blog", blogLines),
  ]
    .filter(Boolean)
    .join("\n");

  return new Response(body, {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
