import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { profileDialogStore } from "../../stores/profile-dialog-store";
import { WorkspaceProfilesHint } from "../workspace-profiles-hint";

vi.mock("../../hooks/use-profiles", () => ({
  useProfiles: vi.fn(() => ({ data: [{ name: "marketing" }] })),
}));

describe("WorkspaceProfilesHint", () => {
  beforeEach(() => {
    profileDialogStore.trigger.closed();
  });

  it("opens one canonical create flow per absent repository profile", () => {
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
});
