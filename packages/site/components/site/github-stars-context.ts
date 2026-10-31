import { getStrictContext } from "@/components/site/strict-context";
import type { GithubStarsContextType } from "@/components/site/use-github-stars-controller";

export const [GithubStarsProvider, useGithubStars] =
  getStrictContext<GithubStarsContextType>("GithubStarsContext");
