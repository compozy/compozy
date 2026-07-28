import { render, screen } from "@testing-library/react";
import { KeyRound } from "lucide-react";
import { describe, expect, it } from "vitest";

import { DataSurface } from "../data-surface";
import { resolveDataSurfaceState } from "../data-surface-state";

function renderSurface(state: React.ComponentProps<typeof DataSurface>["state"]) {
  return render(
    <DataSurface state={state} data-testid="surface">
      <DataSurface.Loading data-testid="surface-loading" />
      <DataSurface.Error
        data-testid="surface-error"
        icon={KeyRound}
        title="Unable to load"
        description="Request failed"
      />
      <DataSurface.Empty
        data-testid="surface-empty"
        icon={KeyRound}
        title="No records"
        description="Create one first."
      />
      <DataSurface.Content data-testid="surface-ready">ready content</DataSurface.Content>
    </DataSurface>
  );
}

describe("DataSurface", () => {
  it.each([
    ["loading", "surface-loading"],
    ["error", "surface-error"],
    ["empty", "surface-empty"],
    ["ready", "surface-ready"],
  ] as const)("Should render only the %s slot", (state, testId) => {
    renderSurface(state);

    expect(screen.getByTestId("surface")).toHaveAttribute("data-state", state);
    expect(screen.getByTestId(testId)).toBeInTheDocument();
    for (const candidate of [
      "surface-loading",
      "surface-error",
      "surface-empty",
      "surface-ready",
    ]) {
      if (candidate !== testId) {
        expect(screen.queryByTestId(candidate)).not.toBeInTheDocument();
      }
    }
  });

  it("Should resolve state using loading, error, empty, ready precedence", () => {
    expect(
      resolveDataSurfaceState({ isLoading: true, error: new Error("fail"), isEmpty: true })
    ).toBe("loading");
    expect(resolveDataSurfaceState({ error: new Error("fail"), isEmpty: true })).toBe("error");
    expect(resolveDataSurfaceState({ isEmpty: true })).toBe("empty");
    expect(resolveDataSurfaceState({})).toBe("ready");
  });
});
