// Suite: Settings provider → runtime-selector option (UT-027)
// Invariant: a provider turned off maps to a `disabled` option, which selectors drop unless it is
// the pinned value; an enabled or legacy (no `enabled`) provider maps without the flag.
import { describe, expect, it } from "vitest";

import { settingsProviderFixtures } from "../../mocks/fixtures";
import { settingsProviderToOption } from "../provider-runtime-option";

const base = settingsProviderFixtures[0]!;

describe("settingsProviderToOption", () => {
  it("Should flag a disabled provider and leave enabled ones unflagged (UT-027)", () => {
    expect(
      settingsProviderToOption({ ...base, settings: { ...base.settings, enabled: false } })
    ).toMatchObject({ id: base.name, disabled: true });
    expect("disabled" in settingsProviderToOption(base)).toBe(false);
    const { enabled: _enabled, ...legacy } = base.settings;
    expect(
      "disabled" in settingsProviderToOption({ ...base, settings: legacy as typeof base.settings })
    ).toBe(false);
  });
});
