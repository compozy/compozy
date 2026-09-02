import {
  githubGraphqlResponseSchema,
  githubReleaseListSchema,
  type GitHubPullRequestPayload,
  type GitHubReleasePayload,
} from "./github-schemas";
import {
  channelFromReleaseTag,
  extractPullRequestNumbers,
  extractReleaseCategories,
  extractReleaseHeadings,
  releaseSummary,
  releaseTagIsAtOrAfter,
  releaseTagIsBefore,
} from "./release-markdown";
import {
  CHANGELOG_CUTOFF_TAG,
  CHANGELOG_REVALIDATE_SECONDS,
  COMPOZY_REPOSITORY,
  type ChangelogContributor,
  type ChangelogLoadError,
  type ChangelogPullRequest,
  type ChangelogPullRequestsResult,
  type ChangelogRelease,
  type ChangelogReleaseLookup,
  type ChangelogReleasesResult,
} from "./types";

const GITHUB_API_VERSION = "2026-03-10";
const RELEASES_PER_PAGE = 25;
const MAX_RELEASE_PAGES = 20;
const PULL_REQUEST_BATCH_SIZE = 50;
const RECENT_RELEASE_LOOKUP_TTL_MS = 60_000;
const MAX_RELEASE_TAG_LENGTH = 64;

interface RecentReleaseLookupCache {
  fetcher: typeof globalThis.fetch;
  expiresAt: number;
  payloads: Promise<GitHubReleasePayload[]>;
}

let recentReleaseLookupCache: RecentReleaseLookupCache | undefined;

class GitHubClientError extends Error {
  constructor(
    readonly kind: ChangelogLoadError["kind"],
    message: string,
    readonly status?: number,
    options?: ErrorOptions
  ) {
    super(message, options);
    this.name = "GitHubClientError";
  }
}

function githubToken(): string | undefined {
  const token = process.env.COMPOZY_SITE_GITHUB_TOKEN?.trim();
  return token ? token : undefined;
}

function githubHeaders({ requireToken }: { requireToken: boolean }): Record<string, string> {
  const token = githubToken();
  if (!token && (requireToken || process.env.VERCEL_ENV === "production")) {
    throw new GitHubClientError(
      "configuration",
      "COMPOZY_SITE_GITHUB_TOKEN is required for this GitHub request"
    );
  }

  return {
    Accept: "application/vnd.github+json",
    "X-GitHub-Api-Version": GITHUB_API_VERSION,
    "User-Agent": "compozy-site",
    ...(token ? { Authorization: `Bearer ${token}` } : {}),
  };
}

async function githubJson(
  url: string,
  init: RequestInit & { next?: { revalidate: number } },
  requireToken: boolean
): Promise<unknown> {
  let response: Response;
  try {
    response = await fetch(url, {
      ...init,
      headers: {
        ...githubHeaders({ requireToken }),
        ...init.headers,
      },
    });
  } catch (error) {
    throw new GitHubClientError("network", "GitHub could not be reached", undefined, {
      cause: error,
    });
  }

  if (!response.ok) {
    const requestId = response.headers.get("x-github-request-id");
    const requestContext = requestId ? ` (request ${requestId})` : "";
    throw new GitHubClientError(
      "github-response",
      `GitHub returned ${response.status}${requestContext}`,
      response.status
    );
  }

  try {
    return await response.json();
  } catch (error) {
    throw new GitHubClientError("invalid-response", "GitHub returned invalid JSON", undefined, {
      cause: error,
    });
  }
}

function toLoadError(error: unknown): ChangelogLoadError {
  if (error instanceof GitHubClientError) {
    return { kind: error.kind, message: error.message, status: error.status };
  }
  if (error instanceof Error) {
    return { kind: "network", message: error.message };
  }
  return { kind: "network", message: "Unknown GitHub request failure" };
}

function buildRelease(payload: GitHubReleasePayload): ChangelogRelease | null {
  if (payload.draft || !payload.published_at) return null;
  if (!releaseTagIsAtOrAfter(payload.tag_name, CHANGELOG_CUTOFF_TAG)) return null;

  const body = payload.body?.trim() ?? "";
  return {
    version: payload.tag_name,
    name: payload.name,
    body,
    summary: releaseSummary(body, payload.name, payload.tag_name),
    publishedAt: payload.published_at,
    channel: channelFromReleaseTag(payload.tag_name, payload.prerelease),
    prerelease: payload.prerelease,
    url: payload.html_url,
    sourceArchiveUrl: payload.tarball_url,
    sourceZipUrl: payload.zipball_url,
    assets: payload.assets.map(asset => ({
      name: asset.name,
      url: asset.browser_download_url,
      size: asset.size,
      contentType: asset.content_type,
      downloadCount: asset.download_count,
    })),
    categories: extractReleaseCategories(body),
    headings: extractReleaseHeadings(body),
    pullRequestNumbers: extractPullRequestNumbers(body),
  };
}

function buildReleases(payloads: readonly GitHubReleasePayload[]): ChangelogRelease[] {
  return payloads
    .map(buildRelease)
    .filter((release): release is ChangelogRelease => release !== null)
    .toSorted((left, right) => right.publishedAt.localeCompare(left.publishedAt));
}

async function fetchReleasePage(page: number): Promise<GitHubReleasePayload[]> {
  const url = new URL(
    `https://api.github.com/repos/${COMPOZY_REPOSITORY.owner}/${COMPOZY_REPOSITORY.name}/releases`
  );
  url.searchParams.set("per_page", String(RELEASES_PER_PAGE));
  url.searchParams.set("page", String(page));

  const payload = await githubJson(
    url.toString(),
    { method: "GET", next: { revalidate: CHANGELOG_REVALIDATE_SECONDS } },
    false
  );
  const parsed = githubReleaseListSchema.safeParse(payload);
  if (!parsed.success) {
    throw new GitHubClientError(
      "invalid-response",
      `GitHub release data failed validation: ${parsed.error.message}`
    );
  }
  return parsed.data;
}

export async function loadChangelogReleases(): Promise<ChangelogReleasesResult> {
  try {
    const payloads: GitHubReleasePayload[] = [];
    for (let page = 1; page <= MAX_RELEASE_PAGES; page += 1) {
      const pagePayloads = await fetchReleasePage(page);
      payloads.push(...pagePayloads);

      const reachedCutoff = pagePayloads.some(payload =>
        releaseTagIsBefore(payload.tag_name, CHANGELOG_CUTOFF_TAG)
      );
      if (pagePayloads.length < RELEASES_PER_PAGE || reachedCutoff) break;
      if (page === MAX_RELEASE_PAGES) {
        throw new GitHubClientError(
          "invalid-response",
          `GitHub release pagination exceeded ${MAX_RELEASE_PAGES} pages`
        );
      }
    }

    return { status: "ready", releases: buildReleases(payloads) };
  } catch (error) {
    return { status: "unavailable", error: toLoadError(error) };
  }
}

async function fetchRecentReleasePayloads(): Promise<GitHubReleasePayload[]> {
  const url = new URL(
    `https://api.github.com/repos/${COMPOZY_REPOSITORY.owner}/${COMPOZY_REPOSITORY.name}/releases`
  );
  url.searchParams.set("per_page", String(RELEASES_PER_PAGE));
  const payload = await githubJson(url.toString(), { method: "GET", cache: "no-store" }, false);
  const parsed = githubReleaseListSchema.safeParse(payload);
  if (!parsed.success) {
    throw new GitHubClientError(
      "invalid-response",
      `GitHub release data failed validation: ${parsed.error.message}`
    );
  }
  return parsed.data;
}

function loadRecentReleasePayloads(): Promise<GitHubReleasePayload[]> {
  const now = Date.now();
  const fetcher = globalThis.fetch;
  if (
    recentReleaseLookupCache &&
    recentReleaseLookupCache.fetcher === fetcher &&
    recentReleaseLookupCache.expiresAt > now
  ) {
    return recentReleaseLookupCache.payloads;
  }

  const payloads = fetchRecentReleasePayloads();
  recentReleaseLookupCache = {
    fetcher,
    expiresAt: now + RECENT_RELEASE_LOOKUP_TTL_MS,
    payloads,
  };
  return payloads;
}

export async function loadChangelogReleaseByTag(tag: string): Promise<ChangelogReleaseLookup> {
  if (
    !tag ||
    tag.length > MAX_RELEASE_TAG_LENGTH ||
    tag !== tag.trim() ||
    !releaseTagIsAtOrAfter(tag, CHANGELOG_CUTOFF_TAG)
  ) {
    return { status: "missing" };
  }

  try {
    const payloads = await loadRecentReleasePayloads();
    const releases = buildReleases(payloads);
    const release = releases.find(candidate => candidate.version === tag);
    return release ? { status: "found", release, releases } : { status: "missing" };
  } catch (error) {
    return { status: "unavailable", error: toLoadError(error) };
  }
}

function pullRequestQuery(numbers: number[]): string {
  const fields = numbers
    .map(
      number => `
        pr_${number}: pullRequest(number: ${number}) {
          number
          title
          url
          mergedAt
          author {
            __typename
            login
            avatarUrl
            url
          }
        }`
    )
    .join("\n");

  return `query ReleasePullRequests {
    repository(owner: "${COMPOZY_REPOSITORY.owner}", name: "${COMPOZY_REPOSITORY.name}") {
      ${fields}
    }
  }`;
}

async function fetchPullRequestBatch(numbers: number[]): Promise<GitHubPullRequestPayload[]> {
  const payload = await githubJson(
    "https://api.github.com/graphql",
    {
      method: "POST",
      body: JSON.stringify({ query: pullRequestQuery(numbers) }),
      cache: "force-cache",
      next: { revalidate: CHANGELOG_REVALIDATE_SECONDS },
    },
    true
  );
  const parsed = githubGraphqlResponseSchema.safeParse(payload);
  if (!parsed.success) {
    throw new GitHubClientError(
      "invalid-response",
      `GitHub pull request data failed validation: ${parsed.error.message}`
    );
  }
  if (parsed.data.errors?.length) {
    throw new GitHubClientError(
      "github-response",
      `GitHub GraphQL returned errors: ${parsed.data.errors.map(error => error.message).join("; ")}`
    );
  }
  if (!parsed.data.data?.repository) {
    throw new GitHubClientError("invalid-response", "GitHub repository data was missing");
  }

  return numbers.flatMap(number => {
    const pullRequest = parsed.data.data?.repository?.[`pr_${number}`];
    return pullRequest?.mergedAt ? [pullRequest] : [];
  });
}

function isHumanAuthor(
  author: GitHubPullRequestPayload["author"]
): author is NonNullable<GitHubPullRequestPayload["author"]> {
  return Boolean(
    author && author.__typename === "User" && !author.login.toLowerCase().endsWith("[bot]")
  );
}

function buildPullRequest(payload: GitHubPullRequestPayload): ChangelogPullRequest {
  const author = isHumanAuthor(payload.author)
    ? {
        login: payload.author.login,
        url: payload.author.url,
        avatarUrl: payload.author.avatarUrl,
      }
    : null;
  return { number: payload.number, title: payload.title, url: payload.url, author };
}

function buildContributors(pullRequests: ChangelogPullRequest[]): ChangelogContributor[] {
  const contributors = new Map<string, ChangelogContributor>();
  for (const pullRequest of pullRequests) {
    if (!pullRequest.author) continue;
    const key = pullRequest.author.login.toLowerCase();
    const existing = contributors.get(key);
    if (existing) {
      existing.pullRequestNumbers.push(pullRequest.number);
      continue;
    }
    contributors.set(key, { ...pullRequest.author, pullRequestNumbers: [pullRequest.number] });
  }
  return Array.from(contributors.values());
}

function chunkNumbers(numbers: number[]): number[][] {
  const batches: number[][] = [];
  for (let index = 0; index < numbers.length; index += PULL_REQUEST_BATCH_SIZE) {
    batches.push(numbers.slice(index, index + PULL_REQUEST_BATCH_SIZE));
  }
  return batches;
}

export async function loadReleasePullRequests(
  inputNumbers: number[]
): Promise<ChangelogPullRequestsResult> {
  const numbers = Array.from(
    new Set(inputNumbers.filter(number => Number.isInteger(number) && number > 0))
  );
  if (numbers.length === 0) {
    return { status: "ready", pullRequests: [], contributors: [], missingNumbers: [] };
  }

  try {
    const payloads = (
      await Promise.all(chunkNumbers(numbers).map(batch => fetchPullRequestBatch(batch)))
    ).flat();
    const byNumber = new Map(payloads.map(payload => [payload.number, payload]));
    const pullRequests = numbers.flatMap(number => {
      const payload = byNumber.get(number);
      return payload ? [buildPullRequest(payload)] : [];
    });
    const foundNumbers = new Set(pullRequests.map(pullRequest => pullRequest.number));
    return {
      status: "ready",
      pullRequests,
      contributors: buildContributors(pullRequests),
      missingNumbers: numbers.filter(number => !foundNumbers.has(number)),
    };
  } catch (error) {
    return { status: "unavailable", error: toLoadError(error) };
  }
}
