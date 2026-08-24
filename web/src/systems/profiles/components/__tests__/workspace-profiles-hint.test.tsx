import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { profileDialogStore } from "../../stores/profile-dialog-store";
import { WorkspaceProfilesHint } from "../workspace-profiles-hint";

vi.mock("@/systems/gateway", () => ({
  useGatewayAccessTier: () => "local",
}));

vi.mock("../../hooks/use-profiles", () => ({
  useProfiles: vi.fn(() => ({ data: [{ name: "marketing" }] })),
}));

describe("WorkspaceProfilesHint", () => {
  beforeEach(() => {
    profileDialogStore.trigger.closed();
  });

  it("Should open one canonical create flow per absent repository profile", () => {
    render(
      <WorkspaceProfilesHint
        hints={[
          { name: "dev", path: ".compozy/profiles/dev", message: "", action: "" },
          {
            name: "marketing",
            path: ".compozy/profiles/marketing",
            message: "",
            action: "",
          },
        ]}
        workspaceId="ws-alpha"
      />
    );

    expect(screen.getByTestId("workspace-profiles-hint")).toHaveTextContent(
      "This project declares content for profile dev."
    );
    expect(screen.queryByText("Create marketing")).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("Create dev"));
    expect(profileDialogStore.getSnapshot().context.intent).toEqual({
      flow: "create",
      profile: "dev",
    });
  });

  it("Should remember dismissed workspace hints independently", () => {
    const hints = [{ name: "dev", path: ".compozy/profiles/dev", message: "", action: "" }];
    const { rerender } = render(<WorkspaceProfilesHint hints={hints} workspaceId="ws-alpha" />);

    fireEvent.click(screen.getByRole("button", { name: "Not now" }));
    expect(screen.queryByTestId("workspace-profiles-hint")).not.toBeInTheDocument();

    rerender(<WorkspaceProfilesHint hints={hints} workspaceId="ws-beta" />);
    expect(screen.getByTestId("workspace-profiles-hint")).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Not now" }));

    rerender(<WorkspaceProfilesHint hints={hints} workspaceId="ws-alpha" />);
    expect(screen.queryByTestId("workspace-profiles-hint")).not.toBeInTheDocument();
  });
});
