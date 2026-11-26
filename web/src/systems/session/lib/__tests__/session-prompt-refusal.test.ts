import { describe, expect, it } from "vitest";

import { sessionPromptRefusalReason } from "../session-prompt-refusal";

describe("session prompt refusal reason", () => {
  it("Should remove the rebind timeout code and sent suffix from the retained-draft reason", () => {
    const reason = sessionPromptRefusalReason(
      JSON.stringify({
        error:
          "The provider session took too long to reload. Retry, or use Continue/Fork to keep the transcript on a fresh provider thread. Your message wasn't sent. (session_rebind_timeout)",
        code: "session_rebind_timeout",
        retryable: true,
      })
    );

    expect(reason).toBe(
      "The provider session took too long to reload. Retry, or use Continue/Fork to keep the transcript on a fresh provider thread"
    );
  });
});
