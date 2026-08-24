// Suite: profiles palette controller
// Invariant: profile state projection and lifecycle delegation stay owned by the profiles domain.
// Boundary IN: cached profiles, remembered selection, and the palette controller.
// Boundary OUT: the OS palette stack chrome and profile mutation dialogs.

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { UserRound } from "lucide-react";

import { UIProvider } from "@compozy/ui";

import { OsPaletteViewShell } from "@/systems/os/components/os-palette-view-shell";
import type { PaletteViewDefinition } from "@/systems/os/lib/palette-view-registry";

vi.mock("@/systems/gateway", () => ({
  useGatewayAccessTier: () => "local",
}));

import { profileKeys } from "../../lib/query-keys";
import { closeProfileDialog, profileDialogStore } from "../../stores/profile-dialog-store";
import { localProfileView, resetProfileViews } from "../../stores/profile-view-store";
import { useProfilesPaletteView } from "../use-profiles-palette-view";

const DEFINITION: PaletteViewDefinition = {
  id: "profiles",
  title: "Profiles",
  icon: UserRound,
  placeholder: "Switch profile…",
  enterHint: "switch",
  description: "Profiles",
};

function ProfilesViewHarness({ query = "" }: { query?: string }) {
  const content = useProfilesPaletteView({
    query,
    lens: { scope: "global" },
    onDismiss: vi.fn(),
  });
  return (
    <OsPaletteViewShell
      breadcrumb={{ truncated: false, visible: ["Profiles"] }}
      content={content}
      definition={DEFINITION}
      query={query}
      onPop={vi.fn()}
      onQueryChange={vi.fn()}
    />
  );
}

function renderProfilesView(query = "") {
  closeProfileDialog();
  resetProfileViews();
  const queryClient = new QueryClient({
    defaultOptions: { mutations: { retry: false }, queries: { retry: false } },
  });
  queryClient.setQueryData(profileKeys.list(), [
    {
      id: "00000000000000000000000000",
      name: "default",
      color: "#8a8f98",
      icon: "user-round",
      emoji: null,
      state: "active",
      created_at: "2026-08-01T09:00:00Z",
      work_items: 8,
    },
    {
      id: "01J9GROWTH0000000000000000",
      name: "growth",
      color: "#4cb782",
      icon: "trending-up",
      emoji: null,
      state: "active",
      created_at: "2026-08-01T09:00:00Z",
      needs_setup: true,
    },
    {
      id: "01J9OLDAGENCY00000000000000",
      name: "old-agency",
      color: "#b58e5f",
      icon: "briefcase",
      emoji: null,
      state: "archived",
      created_at: "2026-08-01T09:00:00Z",
      archived_at: "2026-08-20T09:00:00Z",
      work_items: 0,
    },
  ]);
  queryClient.setQueryData(profileKeys.selection({ scope: "global" }), {
    scope: "global",
    profile: "default",
  });
  const rendered = render(
    <QueryClientProvider client={queryClient}>
      <UIProvider reducedMotion="always">
        <ProfilesViewHarness query={query} />
      </UIProvider>
    </QueryClientProvider>
  );
  return { ...rendered, queryClient };
}

describe("useProfilesPaletteView", () => {
  it("Should map profile states and delegate lifecycle without mutation state", () => {
    renderProfilesView();

    expect(screen.getByText("current")).toBeInTheDocument();
    expect(screen.getByText("needs setup")).toHaveAttribute("data-slot", "os-palette-reason");
    expect(screen.getByText("archived")).toBeInTheDocument();

    fireEvent.click(screen.getByTestId("os-palette-profile-old-agency"));
    expect(profileDialogStore.getSnapshot().context.intent).toEqual({
      flow: "unarchive",
      profile: "old-agency",
    });
  });

  it("Should expose the no-match state without unrelated action rows", () => {
    renderProfilesView("does-not-exist");

    expect(screen.getByText('No profiles match "does-not-exist".')).toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-profile-create")).not.toBeInTheDocument();
    expect(screen.queryByTestId("os-palette-profile-aggregate")).not.toBeInTheDocument();
  });

  it("Should keep aggregate view changes local without invalidating remembered selections", async () => {
    const { queryClient } = renderProfilesView();
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    fireEvent.click(screen.getByTestId("os-palette-profile-aggregate"));

    await waitFor(() =>
      expect(localProfileView({ scope: "global" })).toEqual({ kind: "aggregate" })
    );
    expect(invalidate).not.toHaveBeenCalled();
  });
});
