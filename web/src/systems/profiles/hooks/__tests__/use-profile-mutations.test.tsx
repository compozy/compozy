// Suite: profile lifecycle mutations
// Invariant: a rename stays pending until the caller's reread of caches outside the profiles
// system settles, so no row action can still send the old name once it reports success (#1220).
// Owning layer: web/src/systems/profiles/hooks/use-profile-mutations.ts.
// Boundary OUT: the HTTP rename contract, owned by the adapter and Go profile suites.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, renderHook, waitFor } from "@testing-library/react";
import type { PropsWithChildren } from "react";
import { describe, expect, it, vi } from "vitest";

import { useRenameProfile } from "../use-profile-mutations";

vi.mock("../../adapters/profiles-api", () => ({
  renameProfile: vi.fn(async () => ({})),
}));

describe("useRenameProfile", () => {
  it("Should stay pending until the caller's rename reread settles", async () => {
    const queryClient = new QueryClient({ defaultOptions: { mutations: { retry: false } } });
    const wrapper = ({ children }: PropsWithChildren) => (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
    let settle: () => void = () => undefined;
    const onRenamed = vi.fn(() => new Promise<void>(resolve => (settle = resolve)));
    const { result } = renderHook(() => useRenameProfile({ onRenamed }), { wrapper });

    act(() => result.current.mutate({ name: "a", newName: "b", planRevision: "rev" }));

    await waitFor(() => expect(onRenamed).toHaveBeenCalledTimes(1));
    expect(result.current.isPending).toBe(true);
    act(() => settle());
    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    queryClient.clear();
  });
});
