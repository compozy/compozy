"use client";

import { useEffect, useState } from "react";

import { loadGithubStars } from "@/components/site/github-stars-api";

const OWNER = "compozy";
const REPO = "compozy";

/** One request per page load, shared by every component that shows the count. */
let pending: Promise<number | null> | null = null;

function starCount(): Promise<number | null> {
  pending ??= loadGithubStars(OWNER, REPO, new AbortController().signal).catch(error => {
    console.warn("GitHub star count unavailable", error);
    pending = null;
    return null;
  });
  return pending;
}

/** The live stargazer count of compozy/compozy, or `null` until it loads or when GitHub is unavailable. */
export function useGithubStarCount(): number | null {
  const [stars, setStars] = useState<number | null>(null);
  useEffect(() => {
    let active = true;
    void starCount().then(count => {
      if (active) setStars(count);
    });
    return () => {
      active = false;
    };
  }, []);
  return stars;
}

/** Test seam: forget the shared request. */
export function resetGithubStarCount() {
  pending = null;
}

export const STAR_FORMAT = new Intl.NumberFormat("en-US");
