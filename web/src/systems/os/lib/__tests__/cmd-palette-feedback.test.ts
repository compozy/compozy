// Suite: cmd-palette feedback copy and retry gating
// Invariant: success, approval-pending, and failure copy name the command; retry
// is offered only when the command declares itself retry-safe and the failure
// is not a single-flight rejection.
// Owning layer: web/src/systems/os/lib/cmd-palette-feedback.ts
// Canonical suite: this file (split from the dispatch seam suite).
import { describe, expect, it } from "vitest";

import {
  ALREADY_RUNNING_CODE,
  canRetry,
  invokeCompletedFeedback,
  invokeFailedFeedback,
  workspaceSwitchFeedback,
} from "../cmd-palette-feedback";
import { paletteCommand } from "./cmd-palette-dispatch-fixtures";

describe("cmd-palette feedback copy and retry gating (UT-159, UT-160)", () => {
  const retrySafe = paletteCommand({ id: "app.open.tasks", title: "Open Tasks" });
  const oneShot = paletteCommand({
    id: "ext.notes.purge",
    title: "Purge archived notes",
    execution: { retry_safe: false, single_flight: true },
  });

  it("Should name the command on success", () => {
    expect(
      invokeCompletedFeedback(retrySafe, {
        status: "ok",
        invocation_id: "inv-success",
        profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
      })
    ).toEqual({
      message: "Open Tasks finished",
      tone: "success",
      retryable: false,
    });
  });

  it("Should say an approval is pending rather than claiming the command ran", () => {
    const feedback = invokeCompletedFeedback(oneShot, {
      status: "approval_pending",
      approval_id: "apr_55e0c9",
      invocation_id: "inv-approval",
      profile_lens: { profile_lens_id: "00000000000000000000000000", profile_name: "default" },
    });
    expect(feedback).toEqual({
      message: "Purge archived notes needs approval before it runs",
      tone: "info",
      retryable: false,
    });
  });

  it("Should name the command and repeat the runtime reason verbatim on failure", () => {
    expect(invokeFailedFeedback(oneShot, "runtime unavailable")).toEqual({
      message: "Purge archived notes — runtime unavailable",
      tone: "error",
      retryable: false,
    });
  });

  it("Should offer retry only where re-running is declared safe", () => {
    const safe = paletteCommand({
      ...retrySafe,
      execution: { retry_safe: true, single_flight: false },
    });
    expect(invokeFailedFeedback(safe, "runtime unavailable").retryable).toBe(true);
    expect(invokeFailedFeedback(oneShot, "runtime unavailable").retryable).toBe(false);
  });

  it("Should never offer retry for a single-flight rejection", () => {
    const safe = paletteCommand({
      ...retrySafe,
      execution: { retry_safe: true, single_flight: true },
    });
    expect(canRetry(safe, ALREADY_RUNNING_CODE)).toBe(false);
    expect(
      invokeFailedFeedback(safe, "Open Tasks is already in flight", ALREADY_RUNNING_CODE).retryable
    ).toBe(false);
  });

  it("Should name the workspace a landing switched to", () => {
    expect(workspaceSwitchFeedback("payments", "Fix payment retries")).toEqual({
      message: "Switched to payments to open Fix payment retries",
      tone: "info",
      retryable: false,
    });
  });
});
