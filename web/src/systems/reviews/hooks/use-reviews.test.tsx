import { renderHook } from "@testing-library/react";
import type { QueryObserverResult } from "@tanstack/react-query";
import { afterEach, describe, expect, it } from "vitest";

import { createTestQueryClient, flushAsync, installFetchStub, withQuery } from "@/test/utils";

import { useReviewIssue, useReviewIssues, useReviewRound } from "./use-reviews";

describe("review hooks", () => {
  let restore: (() => void) | null = null;

  afterEach(() => {
    restore?.();
    restore = null;
  });

  it("Should reject non-positive review rounds before issuing review requests", async () => {
    const stub = installFetchStub([]);
    restore = stub.restore;

    const testCases = [
      {
        name: "round detail",
        render: (): QueryObserverResult<unknown, Error> => useReviewRound("ws-1", "alpha", 0),
      },
      {
        name: "issue list",
        render: (): QueryObserverResult<unknown, Error> => useReviewIssues("ws-1", "alpha", 0),
      },
      {
        name: "issue detail",
        render: (): QueryObserverResult<unknown, Error> =>
          useReviewIssue("ws-1", "alpha", 0, "issue_001"),
      },
    ] as const;

    for (const testCase of testCases) {
      const { result, unmount } = renderHook(testCase.render, {
        wrapper: withQuery(createTestQueryClient()),
      });
      await flushAsync();
      expect(result.current.fetchStatus, testCase.name).toBe("idle");
      unmount();
    }

    expect(stub.calls).toHaveLength(0);
  });
});
