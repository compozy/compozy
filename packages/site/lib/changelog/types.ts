export const CHANGELOG_CUTOFF_TAG = "v0.3.0-beta.1";
export const CHANGELOG_REVALIDATE_SECONDS = 300;
export const COMPOZY_REPOSITORY = {
  owner: "compozy",
  name: "compozy",
  url: "https://github.com/compozy/compozy",
} as const;

export type ChangelogChannel = "stable" | "beta" | "alpha" | "prerelease";

export interface ChangelogAsset {
  name: string;
  url: string;
  size: number;
  contentType: string;
  downloadCount: number;
}

export interface ChangelogHeading {
  id: string;
  label: string;
  level: number;
}

export interface ChangelogCategory {
  id: string;
  label: string;
  itemCount: number;
}

export interface ChangelogRelease {
  version: string;
  name: string | null;
  body: string;
  summary: string;
  publishedAt: string;
  channel: ChangelogChannel;
  prerelease: boolean;
  url: string;
  assets: ChangelogAsset[];
  sourceArchiveUrl: string;
  sourceZipUrl: string;
  categories: ChangelogCategory[];
  headings: ChangelogHeading[];
  pullRequestNumbers: number[];
}

export type ChangelogErrorKind =
  | "configuration"
  | "github-response"
  | "invalid-response"
  | "network";

export interface ChangelogLoadError {
  kind: ChangelogErrorKind;
  message: string;
  status?: number;
}

export type ChangelogReleasesResult =
  | { status: "ready"; releases: ChangelogRelease[] }
  | { status: "unavailable"; error: ChangelogLoadError };

export type ChangelogReleaseLookup =
  | { status: "found"; release: ChangelogRelease; releases: ChangelogRelease[] }
  | { status: "missing" }
  | { status: "unavailable"; error: ChangelogLoadError };

export interface ChangelogPullRequestAuthor {
  login: string;
  url: string;
  avatarUrl: string;
}

export interface ChangelogPullRequest {
  number: number;
  title: string;
  url: string;
  author: ChangelogPullRequestAuthor | null;
}

export interface ChangelogContributor extends ChangelogPullRequestAuthor {
  pullRequestNumbers: number[];
}

export type ChangelogPullRequestsResult =
  | {
      status: "ready";
      pullRequests: ChangelogPullRequest[];
      contributors: ChangelogContributor[];
      missingNumbers: number[];
    }
  | { status: "unavailable"; error: ChangelogLoadError };
