import { getStrictContext } from "../../lib/context";
import type { GithubStarsContextType } from "../../hooks/use-github-stars-controller";

export const [GithubStarsProvider, useGithubStars] =
  getStrictContext<GithubStarsContextType>("GithubStarsContext");
