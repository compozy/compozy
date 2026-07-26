import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SettingsSourceBadge } from "../settings-source-badge";

describe("SettingsSourceBadge", () => {
  it("renders the effective source with the overlay label and tone", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "global-config", scope: "global" }}
      />
    );
    const effective = screen.getByTestId("badge-effective");
    expect(effective).toHaveTextContent("CONFIG");
  });

  it("annotates workspace sources with their workspace id", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "workspace-config", scope: "workspace", workspace_id: "ws_alpha" }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent("WORKSPACE · ws_alpha");
  });

  it("shows the builtin label when the source is a daemon builtin", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "builtin-provider", scope: "global" }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent("BUILTIN");
  });

  it("lists shadowed sources when lower precedence definitions exist", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "workspace-config", scope: "workspace", workspace_id: "ws_alpha" }}
        shadowed={[
          { kind: "global-config", scope: "global" },
          { kind: "builtin-provider", scope: "global" },
        ]}
      />
    );
    const shadow = screen.getByTestId("badge-shadowed");
    expect(shadow).toHaveTextContent("shadows");
    expect(shadow).toHaveTextContent("CONFIG");
    expect(shadow).toHaveTextContent("BUILTIN");
  });

  it("includes agent identity for agent-scoped file sources", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{
          kind: "workspace-agent-file",
          scope: "agent",
          agent_name: "reviewer",
          workspace_id: "ws_alpha",
        }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent(
      "WS-AGENT · reviewer · ws_alpha"
    );
  });

  it("omits the shadow group when no lower precedence sources are present", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "global-config", scope: "global" }}
      />
    );
    expect(screen.queryByTestId("badge-shadowed")).not.toBeInTheDocument();
  });
});
