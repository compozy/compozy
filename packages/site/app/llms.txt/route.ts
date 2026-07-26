import { allPosts } from "@/lib/blog";
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

  const runtimeLines = runtimePages.map(page =>
    docLine(page.data.title, page.url, page.data.description)
  );
  const protocolLines = protocolPages.map(page =>
    docLine(page.data.title, page.url, page.data.description)
  );
  const blogLines = posts.map(post => docLine(post.title, post.permalink, post.description));

  const body = [
    "# AGH Documentation",
    "",
    "> An open workplace for AI agents, the runtime, the agh-network/v0 protocol, and the blog.",
    "",
    section("Runtime", runtimeLines),
    section("Network Protocol", protocolLines),
    section("Blog", blogLines),
  ]
    .filter(Boolean)
    .join("\n");

  return new Response(body, {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
