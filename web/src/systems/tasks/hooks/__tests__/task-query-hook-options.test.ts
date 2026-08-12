// Suite: task query hook option composition
// Invariant: explicit polling overrides reach the Query observer unchanged, while omission keeps
// the owning factory policy intact.
// Owning layer: task hook option adapter. Boundary OUT: factory-specific interval policies.
import { describe, expect, it } from "vitest";

import { withTaskQueryHookOptions } from "../task-query-hook-options";

describe("withTaskQueryHookOptions", () => {
  it("Should preserve the query factory interval when no override is supplied", () => {
    const interval = () => 15_000;

    expect(withTaskQueryHookOptions({ refetchInterval: interval }, {})).toEqual({
      refetchInterval: interval,
    });
  });

  it("Should disable the query factory interval when false is supplied", () => {
    expect(
      withTaskQueryHookOptions({ refetchInterval: 15_000 }, { refetchIntervalMs: false })
    ).toEqual({ refetchInterval: false });
  });

  it("Should replace the query factory interval when milliseconds are supplied", () => {
    expect(
      withTaskQueryHookOptions({ refetchInterval: 15_000 }, { refetchIntervalMs: 2_500 })
    ).toEqual({ refetchInterval: 2_500 });
  });
});
