"use client";

import * as React from "react";

import { loadGithubStars } from "./github-stars-api";
import { useIsInView, type UseIsInViewOptions } from "./use-is-in-view";

export interface GithubStarsContextType {
  stars: number;
  setStars: (stars: number) => void;
  currentStars: number;
  setCurrentStars: (stars: number) => void;
  isCompleted: boolean;
  isLoading: boolean;
}

export interface UseGithubStarsControllerParams {
  ref?: React.Ref<HTMLDivElement>;
  username?: string;
  repo?: string;
  value?: number;
  delay: number;
  inView: boolean;
  inViewMargin: UseIsInViewOptions["inViewMargin"];
  inViewOnce: boolean;
}

interface LoadedGithubStars {
  repo: string;
  stars: number;
  username: string;
}

export function useGithubStarsController({
  ref,
  username,
  repo,
  value,
  delay,
  inView,
  inViewMargin,
  inViewOnce,
}: UseGithubStarsControllerParams) {
  const { ref: localRef, isInView } = useIsInView(ref as React.Ref<HTMLDivElement>, {
    inView,
    inViewOnce,
    inViewMargin,
  });

  const [loadedStars, setLoadedStars] = React.useState<LoadedGithubStars | null>(null);
  const [currentStars, setCurrentStars] = React.useState(0);
  const matchedLoadedStars =
    loadedStars !== null && loadedStars.username === username && loadedStars.repo === repo
      ? loadedStars
      : null;
  const stars = value ?? matchedLoadedStars?.stars ?? 0;
  const isLoading =
    value === undefined &&
    (!isInView || (Boolean(username) && Boolean(repo) && matchedLoadedStars === null));
  const setStars = (nextStars: number) => {
    if (!username || !repo) return;
    setLoadedStars({ repo, stars: nextStars, username });
  };
  const isCompleted = currentStars === stars;

  React.useEffect(() => {
    if (value !== undefined || !isInView || !username || !repo) return undefined;

    const controller = new AbortController();
    const timeout = setTimeout(() => {
      void loadGithubStars(username, repo, controller.signal)
        .then(nextStars => {
          if (controller.signal.aborted) return;
          setLoadedStars({ repo, stars: nextStars ?? 0, username });
        })
        .catch(error => {
          if (controller.signal.aborted) return;
          if (error instanceof DOMException && error.name === "AbortError") return;
          console.error("Failed to load GitHub repository stars", error);
          setLoadedStars({ repo, stars: 0, username });
        });
    }, delay);

    return () => {
      clearTimeout(timeout);
      controller.abort();
    };
  }, [username, repo, value, isInView, delay]);

  const contextValue: GithubStarsContextType = {
    stars,
    currentStars,
    isCompleted,
    isLoading,
    setStars,
    setCurrentStars,
  };

  return { localRef, isLoading, contextValue };
}
