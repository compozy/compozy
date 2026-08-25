// Suite: active profile view selection
// Invariant: changing profile lenses carries the source lens's local view before any consumer
// commits a destination-scoped read.
// Owning layer: useActiveProfileView.
// Boundary IN: profile lens changes and profile-view store entries.
// Boundary OUT: profile API persistence and profile consumers.
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, waitFor } from "@testing-library/react";
import { useLayoutEffect, type ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { profileLensKey } from "../../lib/query-keys";
import {
  profileViewStore,
  resetProfileViews,
  setProfileView,
} from "../../stores/profile-view-store";
import type { ProfileLens, ProfileView } from "../../types";
import { useActiveProfileView } from "../use-profile-selection";

const GLOBAL_LENS: ProfileLens = { scope: "global" };
const WORKSPACE_LENS: ProfileLens = { scope: "workspace", workspaceId: "ws-2" };

function ViewCommitProbe({
  lens,
  onCommit,
}: {
  lens: ProfileLens;
  onCommit: (view: ProfileView) => void;
}) {
  const view = useActiveProfileView(lens, false);
  const lensKey = profileLensKey(lens);

  useLayoutEffect(() => onCommit(view), [lensKey, onCommit, view]);
  return null;
}

function createWrapper(queryClient: QueryClient) {
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  );
}

describe("useActiveProfileView", () => {
  beforeEach(() => resetProfileViews());

  it("Should preserve the source local view throughout a lens transition", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const onCommit = vi.fn();
    setProfileView(GLOBAL_LENS, { kind: "profile", profile: "marketing" });
    setProfileView(WORKSPACE_LENS, { kind: "profile", profile: "engineering" });
    const Wrapper = createWrapper(queryClient);
    const view = render(
      <Wrapper>
        <ViewCommitProbe lens={GLOBAL_LENS} onCommit={onCommit} />
      </Wrapper>
    );

    view.rerender(
      <Wrapper>
        <ViewCommitProbe lens={WORKSPACE_LENS} onCommit={onCommit} />
      </Wrapper>
    );

    await waitFor(() =>
      expect(
        profileViewStore.getSnapshot().context.viewByLens[profileLensKey(WORKSPACE_LENS)]
      ).toEqual({ kind: "profile", profile: "marketing" })
    );
    expect(onCommit).toHaveBeenCalled();
    expect(onCommit.mock.calls.map(([committedView]) => committedView)).toEqual(
      expect.arrayContaining([{ kind: "profile", profile: "marketing" }])
    );
    expect(
      onCommit.mock.calls.every(([committedView]) => committedView.profile === "marketing")
    ).toBe(true);
  });
});
