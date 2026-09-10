// Suite: Settings save-state derivation
// Invariant: an invalid raw field always exposes the blocking save-bar state.
// Boundary IN: pure save-state priority and projection.
// Boundary OUT: rendered controls and settings persistence.
import { describe, expect, it } from "vitest";

import { deriveSettingsSaveBarState } from "../save-state";

describe("deriveSettingsSaveBarState", () => {
  it.each([false, true])("preserves recovery actions with validation=%s", isInvalid => {
    expect(
      deriveSettingsSaveBarState({
        isDirty: true,
        isInvalid,
        isSaving: false,
        error: "Save failed",
        showSaved: false,
      })
    ).toEqual({ kind: "error", message: "Save failed", canRetry: !isInvalid, canDiscard: true });
  });
  it("projects invalid input even when the last valid draft is unchanged", () => {
    expect(
      deriveSettingsSaveBarState({
        isDirty: false,
        isInvalid: true,
        isSaving: false,
        showSaved: false,
      })
    ).toEqual({ kind: "invalid", warnings: [] });
  });
});
