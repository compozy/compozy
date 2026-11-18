"use client";

/** Live numbers on the landing: they count up once loaded and show a dash while the source is unavailable. */
import { useGithubStarCount } from "@/components/site/hooks/use-github-star-count";
import { CountUp } from "@/components/site/motion";

export function LiveCount({ value }: { value: number | null }) {
  return value === null ? <span aria-label="unavailable">—</span> : <CountUp value={value} />;
}

/** Stargazers of compozy/compozy, shared with the header's star button. */
export function LiveStars() {
  return <LiveCount value={useGithubStarCount()} />;
}
