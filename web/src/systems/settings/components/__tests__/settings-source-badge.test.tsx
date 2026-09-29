import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SettingsSourceBadge } from "../settings-source-badge";

describe("SettingsSourceBadge", () => {
  it("renders the effective source with the overlay label and tone", () => {
    render(
      <SettingsSourceBadge data-testid="badge" source={{ kind: "global-config", scope: "user" }} />
    );
    const effective = screen.getByTestId("badge-effective");
    expect(effective).toHaveTextContent("From settings");
    expect(effective).toHaveAttribute("title", "config.toml");
  });

  it("annotates workspace sources with their workspace id", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "workspace-config", scope: "workspace", workspace_id: "ws_alpha" }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent("From project · ws_alpha");
  });

  it("shows profile and workspace-profile sources with their owner identity", () => {
    const { rerender } = render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "profile-config", scope: "profile", profile: "marketing" }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent("From profile · marketing");

    rerender(
      <SettingsSourceBadge
        data-testid="badge"
        source={{
          kind: "workspace-profile-config",
          scope: "profile",
          workspace_id: "ws_alpha",
          profile: "marketing",
        }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent(
      "From project profile · ws_alpha · marketing"
    );
  });

  it.each([
    {
      kind: "profile-mcp-sidecar" as const,
      label: "From profile · marketing",
      tone: "info",
      workspace_id: undefined,
    },
    {
      kind: "workspace-profile-mcp-sidecar" as const,
      label: "From project profile · ws_alpha · marketing",
      tone: "warning",
      workspace_id: "ws_alpha",
    },
  ])("shows $kind with its profile owner and $tone tone", entry => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{
          kind: entry.kind,
          scope: "profile",
          profile: "marketing",
          ...(entry.workspace_id ? { workspace_id: entry.workspace_id } : {}),
        }}
      />
    );

    expect(screen.getByTestId("badge-effective")).toHaveTextContent(entry.label);
    expect(screen.getByTestId("badge-effective")).toHaveAttribute("data-tone", entry.tone);
  });

  it("shows the builtin label when the source is a daemon builtin", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "builtin-provider", scope: "user" }}
      />
    );
    expect(screen.getByTestId("badge-effective")).toHaveTextContent("Built in");
  });

  it("lists shadowed sources when lower precedence definitions exist", () => {
    render(
      <SettingsSourceBadge
        data-testid="badge"
        source={{ kind: "workspace-config", scope: "workspace", workspace_id: "ws_alpha" }}
        shadowed={[
          { kind: "global-config", scope: "user" },
          { kind: "builtin-provider", scope: "user" },
        ]}
      />
    );
    const shadow = screen.getByTestId("badge-shadowed");
    expect(shadow).toHaveTextContent("replaces");
    expect(shadow).toHaveTextContent("From settings");
    expect(shadow).toHaveTextContent("Built in");
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
      "From project agent file · reviewer · ws_alpha"
    );
  });

  it("omits the shadow group when no lower precedence sources are present", () => {
    render(
      <SettingsSourceBadge data-testid="badge" source={{ kind: "global-config", scope: "user" }} />
    );
    expect(screen.queryByTestId("badge-shadowed")).not.toBeInTheDocument();
  });
});
