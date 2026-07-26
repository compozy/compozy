import type { Filter } from "@agh/ui";
import { describe, expect, it, vi } from "vitest";

import {
  applyVaultFilterChips,
  buildVaultFilterFields,
  vaultFiltersToChips,
} from "../vault-list-filters";

describe("vault-list-filters", () => {
  it("Should expose one namespace select field with all vault namespaces", () => {
    const fields = buildVaultFilterFields();
    expect(fields).toHaveLength(1);
    const [field] = fields;
    expect(field).toMatchObject({ key: "namespace", type: "select" });
    const values = "options" in field ? (field.options ?? []).map(option => option.value) : [];
    expect(values).toContain("sessions");
    expect(values).toContain("providers");
  });

  it("Should emit no chips when the namespace filter is 'all'", () => {
    expect(vaultFiltersToChips({ namespace: "all" })).toEqual([]);
  });

  it("Should emit an is-chip for a concrete namespace", () => {
    const chips = vaultFiltersToChips({ namespace: "sessions" });
    expect(chips).toEqual([
      { id: "vault-filter-namespace", field: "namespace", operator: "is", values: ["sessions"] },
    ]);
  });

  it("Should map a namespace chip back to setNamespace", () => {
    const onNamespaceChange = vi.fn();
    const chips: Filter<string>[] = [
      { id: "vault-filter-namespace", field: "namespace", operator: "is", values: ["providers"] },
    ];
    applyVaultFilterChips(chips, { onNamespaceChange });
    expect(onNamespaceChange).toHaveBeenCalledWith("providers");
  });

  it("Should reset to 'all' when the namespace chip is cleared or invalid", () => {
    const onNamespaceChange = vi.fn();
    applyVaultFilterChips([], { onNamespaceChange });
    expect(onNamespaceChange).toHaveBeenLastCalledWith("all");

    applyVaultFilterChips([{ id: "x", field: "namespace", operator: "is", values: ["nope"] }], {
      onNamespaceChange,
    });
    expect(onNamespaceChange).toHaveBeenLastCalledWith("all");
  });
});
