import { githubReleaseListSchema } from "../changelog/github-schemas";
import { CHANGELOG_REVALIDATE_SECONDS, COMPOZY_REPOSITORY } from "../changelog/types";

/**
 * Same page size, API version and headers as the changelog loader, so the pages both read share
 * one fetch-cache entry; the changelog stops at its cutoff tag, this count reads to the end.
 */
const PER_PAGE = 25;
const MAX_PAGES = 20;
const GITHUB_API_VERSION = "2026-03-10";

function headers(): Record<string, string> | null {
  const token = process.env.COMPOZY_SITE_GITHUB_TOKEN?.trim();
  // Production reads GitHub with the site token only, like the changelog.
  if (!token && process.env.VERCEL_ENV === "production") return null;
  return {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": GITHUB_API_VERSION,
    "User-Agent": "compozy-site",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

async function releasePage(page: number, init: RequestInit) {
  const url = new URL(
    `https://api.github.com/repos/${COMPOZY_REPOSITORY.owner}/${COMPOZY_REPOSITORY.name}/releases`
  );
  url.searchParams.set("per_page", String(PER_PAGE));
  url.searchParams.set("page", String(page));
  const response = await fetch(url.toString(), init);
  if (!response.ok) {
    console.warn(`Release count unavailable: GitHub answered ${response.status} for page ${page}`);
    return null;
  }
  const parsed = githubReleaseListSchema.safeParse(await response.json());
  if (!parsed.success) {
    console.warn(`Release count unavailable: unexpected release list on page ${page}`);
    return null;
  }
  // A page can come back short (GitHub hides some entries), so only the Link header ends the list.
  const more = /<[^>]+>;\s*rel="next"/.test(response.headers.get("link") ?? "");
  return { releases: parsed.data, more };
}

/**
 * Every published CompozyOS release on GitHub, betas included, with no changelog cutoff. `null`
 * when GitHub cannot be read, so the page shows the gap instead of a wrong number.
 */
export async function countPublishedReleases(): Promise<number | null> {
  const requestHeaders = headers();
  if (!requestHeaders) return null;
  const init = {
    method: "GET",
    headers: requestHeaders,
    next: { revalidate: CHANGELOG_REVALIDATE_SECONDS },
  };
  let count = 0;
  try {
    for (let page = 1; page <= MAX_PAGES; page += 1) {
      const result = await releasePage(page, init);
      if (!result) return null;
      count += result.releases.filter(
        release => !release.draft && release.published_at !== null
      ).length;
      if (!result.more) return count;
    }
  } catch (error) {
    console.warn("Release count unavailable: GitHub request failed", error);
    return null;
  }
  // More pages than the cap: the count would be short, so it is not shown.
  return null;
}
