import { describe, expect, it } from "vitest";

import type { SettingsUpdateStatus } from "../../types";
import { settingsUpdateView } from "../update-presentation";

const snapshot: SettingsUpdateStatus = {
  available: true,
  checked_at: "2026-07-21T12:00:00Z",
  current_version: "v1.0.0",
  install_method: "direct-binary",
  latest_version: "v1.1.0",
  managed: false,
  status: "available",
  supported: true,
};

describe("settingsUpdateView", () => {
  it("projects checking, unavailable, and initial error states without inventing install data", () => {
    expect(settingsUpdateView({ isLoading: true, isError: false, error: null })).toEqual({
      kind: "checking",
    });
    expect(settingsUpdateView({ isLoading: false, isError: false, error: null })).toEqual({
      kind: "unavailable",
      message: "Update status is unavailable.",
    });
    expect(
      settingsUpdateView({ isLoading: false, isError: true, error: new Error("offline") })
    ).toEqual({ kind: "error", message: "offline" });
  });

  it("preserves the last snapshot and identifies a failed refresh", () => {
    expect(
      settingsUpdateView({
        data: snapshot,
        error: new Error("offline"),
        isError: true,
        isLoading: false,
      })
    ).toEqual({ kind: "snapshot", snapshot, refreshError: "offline" });
  });

  it("projects a successful snapshot without an error", () => {
    expect(
      settingsUpdateView({ data: snapshot, error: null, isError: false, isLoading: false })
    ).toEqual({ kind: "snapshot", snapshot, refreshError: null });
  });
});
