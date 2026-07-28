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

  const [loadedStars, setLoadedStars] = React.useState(0);
  const [currentStars, setCurrentStars] = React.useState(0);
  const [remoteIsLoading, setRemoteIsLoading] = React.useState(true);
  const stars = value ?? loadedStars;
  const isLoading = value === undefined ? remoteIsLoading : false;
  const setStars = (nextStars: number) => setLoadedStars(nextStars);
  const isCompleted = currentStars === stars;

  React.useEffect(() => {
    if (value !== undefined) return;
    if (!isInView) {
      setLoadedStars(0);
      setRemoteIsLoading(true);
      return;
    }
    setLoadedStars(0);
    if (!username || !repo) {
      setRemoteIsLoading(false);
      return;
    }
    setRemoteIsLoading(true);

    const controller = new AbortController();
    const timeout = setTimeout(() => {
      void loadGithubStars(username, repo, controller.signal)
        .then(nextStars => {
          if (nextStars !== null) {
            setLoadedStars(nextStars);
          }
        })
        .catch(error => {
          if (!(error instanceof DOMException && error.name === "AbortError")) {
            console.error("Failed to load GitHub repository stars", error);
          }
        })
        .finally(() => {
          if (!controller.signal.aborted) setRemoteIsLoading(false);
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
