// Suite: reload after a product update (useReloadOnProductVersionChange)
// Invariant: the page reloads exactly when the daemon settles on a version other
// than the one this page first saw — never mid-operation, never on the first
// snapshot, and never when the version comes back unchanged.
// Boundary IN: the hook's decision to call window.location.reload.
// Boundary OUT: the product fold itself (update-presentation.test.ts).
import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  settingsUpdateApplyingFixture,
  settingsUpdateBothAvailableFixture,
  settingsUpdateRolledBackFixture,
  settingsUpdateStatusFixture,
} from "../../mocks/settings-update-fixture";
import type { SettingsUpdateStatus } from "../../types";
import { useReloadOnProductVersionChange } from "../use-product-update";

const originalLocation = window.location;
let reload: ReturnType<typeof vi.fn>;

beforeEach(() => {
  reload = vi.fn();
  Object.defineProperty(window, "location", {
    configurable: true,
    value: { ...originalLocation, reload },
  });
});

afterEach(() => {
  Object.defineProperty(window, "location", { configurable: true, value: originalLocation });
});

function renderReload(initial: SettingsUpdateStatus | undefined) {
  return renderHook(
    ({ snapshot }: { snapshot: SettingsUpdateStatus | undefined }) =>
      useReloadOnProductVersionChange(snapshot),
    { initialProps: { snapshot: initial } }
  );
}

describe("useReloadOnProductVersionChange", () => {
  it("Should not reload on the first snapshot this page sees", () => {
    renderReload(settingsUpdateStatusFixture);

    expect(reload).not.toHaveBeenCalled();
  });

  it("Should wait while the operation that changed the version is still running", () => {
    const { rerender } = renderReload(settingsUpdateBothAvailableFixture);

    rerender({
      snapshot: {
        ...settingsUpdateApplyingFixture,
        runtime: { ...settingsUpdateApplyingFixture.runtime, current_version: "0.5.1" },
      },
    });

    expect(reload).not.toHaveBeenCalled();
  });

  it("Should reload once the daemon settles on a new version", () => {
    const { rerender } = renderReload(settingsUpdateBothAvailableFixture);

    rerender({ snapshot: settingsUpdateStatusFixture });

    expect(reload).toHaveBeenCalledTimes(1);
  });

  it("Should not reload when a failed update leaves the original version running", () => {
    const { rerender } = renderReload(settingsUpdateBothAvailableFixture);

    rerender({ snapshot: settingsUpdateApplyingFixture });
    rerender({ snapshot: settingsUpdateRolledBackFixture });

    expect(reload).not.toHaveBeenCalled();
  });
});
