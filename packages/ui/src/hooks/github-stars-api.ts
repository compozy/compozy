interface GithubRepositoryResponse {
  stargazers_count?: unknown;
}

export async function loadGithubStars(
  username: string,
  repo: string,
  signal: AbortSignal
): Promise<number | null> {
  const response = await fetch(`https://api.github.com/repos/${username}/${repo}`, { signal });
  if (!response.ok) {
    throw new Error(`GitHub repository request failed with status ${response.status}`);
  }
  const data = (await response.json()) as GithubRepositoryResponse;
  return typeof data.stargazers_count === "number" ? data.stargazers_count : null;
}
