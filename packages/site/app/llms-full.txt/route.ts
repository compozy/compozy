import { getLLMText } from "@/lib/get-llm-text";
import { docsSource } from "@/lib/source";
import { loadChangelogReleases } from "@/lib/changelog/github-client";

export const revalidate = 300;

const PAGES = docsSource.getPages();

export async function GET() {
  const [sections, changelog] = await Promise.all([
    Promise.all(PAGES.map(page => getLLMText(page))),
    loadChangelogReleases(),
  ]);
  const releaseSections =
    changelog.status === "ready"
      ? changelog.releases.map(
          release =>
            `# Changelog ${release.version}\n\nPublished: ${release.publishedAt}\n\n${release.body}`
        )
      : [];
  return new Response([...sections, ...releaseSections].join("\n\n---\n\n"), {
    headers: { "Content-Type": "text/plain; charset=utf-8" },
  });
}
