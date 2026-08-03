import { z } from "zod";

export const githubReleaseAssetSchema = z.object({
  name: z.string().min(1),
  browser_download_url: z.url(),
  size: z.number().int().nonnegative(),
  content_type: z.string(),
  download_count: z.number().int().nonnegative(),
});

export const githubReleaseSchema = z.object({
  tag_name: z.string().min(1),
  name: z.string().nullable(),
  body: z.string().nullable(),
  html_url: z.url(),
  published_at: z.iso.datetime().nullable(),
  prerelease: z.boolean(),
  draft: z.boolean(),
  tarball_url: z.url(),
  zipball_url: z.url(),
  assets: z.array(githubReleaseAssetSchema),
});

export const githubReleaseListSchema = z.array(githubReleaseSchema);

const githubPullRequestAuthorSchema = z.object({
  __typename: z.string(),
  login: z.string().min(1),
  avatarUrl: z.url(),
  url: z.url(),
});

export const githubPullRequestSchema = z.object({
  number: z.number().int().positive(),
  title: z.string().min(1),
  url: z.url(),
  mergedAt: z.iso.datetime().nullable(),
  author: githubPullRequestAuthorSchema.nullable(),
});

export const githubGraphqlResponseSchema = z.object({
  data: z
    .object({
      repository: z.record(z.string(), githubPullRequestSchema.nullable()).nullable(),
    })
    .nullable(),
  errors: z.array(z.looseObject({ message: z.string() })).optional(),
});

export type GitHubReleasePayload = z.infer<typeof githubReleaseSchema>;
export type GitHubPullRequestPayload = z.infer<typeof githubPullRequestSchema>;
