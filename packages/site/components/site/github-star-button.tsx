"use client";

import {
  Button,
  GithubStars,
  GithubStarsIcon,
  GithubStarsNumber,
  GithubStarsParticles,
} from "@compozy/ui";
import { GithubLogo } from "@compozy/ui/logos";
import { Star } from "lucide-react";
import Link from "next/link";

const GITHUB_REPO_URL = "https://github.com/compozy/compozy";

export function GitHubStarButton() {
  return (
    <GithubStars username="compozy" repo="compozy" inViewOnce={false}>
      <Button
        nativeButton={false}
        render={
          <Link
            href={GITHUB_REPO_URL}
            target="_blank"
            rel="noopener noreferrer"
            aria-label="Star Compozy on GitHub"
          />
        }
        variant="outline"
        className="gap-2 rounded-full border-line text-muted hover:bg-hover hover:text-fg"
      >
        <GithubLogo aria-hidden className="size-4" />
        <span>Star on GitHub</span>
        <GithubStarsNumber />
        <GithubStarsParticles>
          <GithubStarsIcon
            icon={Star}
            className="h-4 w-4"
            color="#D6A647"
            activeClassName="[fill:#D6A647] [color:#D6A647]"
          />
        </GithubStarsParticles>
      </Button>
    </GithubStars>
  );
}
