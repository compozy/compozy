import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { loadGithubStars } from "../github-stars-api";
import { useGithubStarsController } from "../use-github-stars-controller";

vi.mock("../github-stars-api", () => ({
  loadGithubStars: vi.fn(),
}));

describe("useGithubStarsController", () => {
  beforeEach(() => {
    vi.mocked(loadGithubStars).mockReset();
  });

  it("Should clear a fetched count when the repository identity becomes unavailable", async () => {
    vi.mocked(loadGithubStars).mockResolvedValue(42);
    const initialProps: { username?: string; repo?: string } = {
      username: "compozy",
      repo: "agh",
    };
    const { result, rerender } = renderHook(
      ({ username, repo }: { username?: string; repo?: string }) =>
        useGithubStarsController({
          username,
          repo,
          delay: 0,
          inView: false,
          inViewMargin: "0px",
          inViewOnce: true,
        }),
      { initialProps }
    );

    await waitFor(() => expect(result.current.contextValue.stars).toBe(42));

    rerender({ username: undefined, repo: undefined });

    await waitFor(() => {
      expect(result.current.contextValue.stars).toBe(0);
      expect(result.current.isLoading).toBe(false);
    });
  });
});
