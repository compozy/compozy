// Suite: Extension kit inventory panel
// Invariant: The panel distinguishes shipped kit resources from live ones, renders its empty state,
// and states what the daemon skipped instead of leaving the degradation invisible.
// Boundary IN: ExtensionKitInventoryPanel and ExtensionSkippedComponents rendering from one inventory payload.
// Boundary OUT: Inventory transport/query behavior, owned by adapters/__tests__/extensions-api.test.ts and hooks/__tests__/use-extensions.test.tsx.
import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ExtensionKitInventoryPanel } from "../extension-kit-inventory-panel";
import { ExtensionSkippedComponents } from "../extension-skipped-components";
import type { ExtensionInventoryDiagnostic } from "../../lib/extension-skipped-diagnostics";

describe("ExtensionKitInventoryPanel", () => {
  // UT-063: this panel is where operators distinguish shipped kit resources from live ones.
  it("Should render kit resources with their kind and live state", () => {
    render(
      <ExtensionKitInventoryPanel
        error={null}
        isLoading={false}
        items={[
          {
            id: "automation:weekly-audit",
            kind: "automation",
            live: true,
            name: "weekly-audit",
          },
          { id: "agent:dep-reviewer", kind: "agent", live: false, name: "dep-reviewer" },
        ]}
        onRetry={() => undefined}
      />
    );

    const panel = screen.getByTestId("extension-kit-inventory");
    const rows = within(panel).getAllByTestId("extension-kit-inventory-item");
    expect(rows).toHaveLength(2);
    expect(within(panel).getByText("agent")).toBeInTheDocument();
    expect(within(panel).getByText("automation")).toBeInTheDocument();
    expect(within(rows[0]!).getByText("dep-reviewer")).toBeInTheDocument();
    expect(within(rows[0]!).getByText("shipped")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("weekly-audit")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("live")).toBeInTheDocument();
  });

  it("Should state that no kit resources ship rather than render an empty panel", () => {
    render(
      <ExtensionKitInventoryPanel
        error={null}
        isLoading={false}
        items={[]}
        onRetry={() => undefined}
      />
    );

    expect(
      within(screen.getByTestId("extension-kit-inventory")).getByText(
        "This extension ships no static kit resources."
      )
    ).toBeInTheDocument();
  });
});

const skippedSSEServer: ExtensionInventoryDiagnostic = {
  category: "extension",
  code: "extension_agent_plugin_component_skipped",
  data_freshness: "live",
  evidence: { component: "mcp_server", scope: "mcp:legacy-events" },
  id: "extension/acme.tools/agent-plugin/mcp:legacy-events",
  message: 'mcp server "legacy-events": unsupported transport "sse"',
  severity: "warn",
  title: "Component skipped",
};

const skippedSkill: ExtensionInventoryDiagnostic = {
  category: "extension",
  code: "extension_agent_plugin_component_skipped",
  data_freshness: "live",
  evidence: { component: "skill", scope: "skill:release-notes" },
  id: "extension/acme.tools/agent-plugin/skill:release-notes",
  message: 'skill "release-notes": SKILL.md frontmatter is missing a description',
  severity: "warn",
  title: "Component skipped",
};

const unhealthyServer: ExtensionInventoryDiagnostic = {
  category: "extension",
  code: "extension_mcp_server_unhealthy",
  data_freshness: "live",
  evidence: { extension_name: "acme.tools", server_name: "tools-api" },
  id: "extension.mcp.acme.tools.tools-api.unhealthy",
  message: 'MCP server "tools-api" is unavailable: dial tcp 127.0.0.1:9999: connection refused',
  severity: "error",
  title: "Extension MCP server is unhealthy",
};

const provenanceWarning: ExtensionInventoryDiagnostic = {
  category: "extension",
  code: "extension_checksum_unverified",
  data_freshness: "recorded",
  id: "extension/acme.tools/checksum",
  message: "The package checksum was not verified.",
  severity: "warn",
  title: "Checksum unverified",
};

describe("ExtensionSkippedComponents", () => {
  // UT-049: a skip only exists because the daemon recorded it, so each row must carry that record's
  // own scope and reason rather than a summary the operator cannot act on.
  it("Should list each recorded skip with its scope and reason in the daemon's order", () => {
    render(
      <ExtensionSkippedComponents
        diagnostics={[skippedSSEServer, skippedSkill, unhealthyServer]}
        ingestedCount={2}
      />
    );

    const section = screen.getByTestId("extension-skipped-components");
    const rows = within(section).getAllByTestId("extension-skipped-row");
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByText("mcp: legacy-events")).toBeInTheDocument();
    expect(within(rows[0]!).getByText('unsupported transport "sse"')).toBeInTheDocument();
    expect(within(rows[1]!).getByText("skill: release-notes")).toBeInTheDocument();
    expect(
      within(rows[1]!).getByText("SKILL.md frontmatter is missing a description")
    ).toBeInTheDocument();
    // Live runtime entries follow the ingest set, keyed by the server they name.
    expect(within(rows[2]!).getByText("mcp: tools-api")).toBeInTheDocument();
    expect(
      within(rows[2]!).getByText("dial tcp 127.0.0.1:9999: connection refused")
    ).toBeInTheDocument();
    expect(
      within(section).queryByTestId("extension-skipped-zero-resources")
    ).not.toBeInTheDocument();
  });

  // The section is a compact list of skipped or unavailable components, so it stays in the warning
  // register while preserving the daemon's severity text.
  it("Should keep every row in the warning register while printing its recorded severity", () => {
    render(
      <ExtensionSkippedComponents
        diagnostics={[skippedSSEServer, unhealthyServer]}
        ingestedCount={1}
      />
    );

    const rows = screen.getAllByTestId("extension-skipped-row");
    expect(within(rows[0]!).getByText("warn")).toBeInTheDocument();
    expect(within(rows[1]!).getByText("error")).toBeInTheDocument();
    for (const row of rows) {
      const pill = row.querySelector("[data-tone]");
      expect(pill).toHaveAttribute("data-tone", "warning");
    }
  });

  it("Should render no section for an extension with zero recorded diagnostics", () => {
    const view = render(<ExtensionSkippedComponents diagnostics={[]} ingestedCount={4} />);
    expect(screen.queryByTestId("extension-skipped-components")).not.toBeInTheDocument();

    view.rerender(<ExtensionSkippedComponents diagnostics={undefined} ingestedCount={4} />);
    expect(screen.queryByTestId("extension-skipped-components")).not.toBeInTheDocument();

    view.rerender(
      <ExtensionSkippedComponents diagnostics={[provenanceWarning]} ingestedCount={4} />
    );
    expect(screen.queryByTestId("extension-skipped-components")).not.toBeInTheDocument();
  });

  it("Should let a fully degraded composition replace the native empty state", () => {
    render(
      <>
        <ExtensionKitInventoryPanel
          error={null}
          isLoading={false}
          items={[]}
          onRetry={() => undefined}
          showEmptyState={false}
        />
        <ExtensionSkippedComponents diagnostics={[skippedSSEServer]} ingestedCount={0} />
      </>
    );

    expect(
      screen.queryByText("This extension ships no static kit resources.")
    ).not.toBeInTheDocument();
    expect(screen.getByTestId("extension-skipped-zero-resources")).toHaveTextContent(
      "0 resources ingested"
    );
  });

  // A fully degraded install must say so: the reasons alone would leave the zero count implicit.
  it("Should state that nothing was ingested when every component was skipped", () => {
    render(
      <ExtensionSkippedComponents
        diagnostics={[skippedSSEServer, skippedSkill]}
        ingestedCount={0}
      />
    );

    expect(screen.getByTestId("extension-skipped-zero-resources")).toHaveTextContent(
      "0 resources ingested"
    );
    expect(screen.getAllByTestId("extension-skipped-row")).toHaveLength(2);
  });
});
