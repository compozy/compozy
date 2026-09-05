import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SessionRuntimeRenderProvider } from "../../lib/session-runtime-render-context";
import type { CompozyPermissionData, SessionInteractionRecord } from "../../types";
import { PermissionDataPart } from "../permission-data-part";

const basePermissionData: CompozyPermissionData = {
  type: "permission",
  session_id: "sess-001",
  turn_id: "turn-001",
  request_id: "req-123",
  title: "Bash",
  action: "execute",
  resource: "rm -rf /tmp/test",
  raw: { tool_input: { command: "rm -rf /tmp/test" } },
};

const terminalExecData: CompozyPermissionData = {
  ...basePermissionData,
  title: "compozy__terminal_exec",
  raw: {
    tool_id: "compozy__terminal_exec",
    tool_input: {
      command: "rm",
      args: ["-rf", "/var/lib/atlas/journal-backups"],
      cwd: "~/dev/atlas-api",
      risk: "ordinary",
    },
  },
};

/** The daemon's resolved interaction row — the only evidence of who decided. */
function resolvedRow(
  resolvedBy: string,
  overrides: Partial<SessionInteractionRecord> = {}
): SessionInteractionRecord {
  return {
    interaction_id: "int-123",
    kind: "permission",
    provider_request_id: "req-123",
    turn_id: "turn-001",
    status: "resolved",
    created_at: "2026-09-05T10:00:00Z",
    resolved_at: "2026-09-05T10:02:00Z",
    resolution: "reject-once",
    resolved_by: resolvedBy,
    ...overrides,
  };
}

function renderWithResolvedRow(data: CompozyPermissionData, row: SessionInteractionRecord) {
  return render(
    <SessionRuntimeRenderProvider
      resolvedInteractions={new Map([[row.provider_request_id, row]])}
      sessionId="sess-001"
      workspaceId="ws_alpha"
    >
      <PermissionDataPart data={data} />
    </SessionRuntimeRenderProvider>
  );
}

describe("PermissionDataPart", () => {
  it("Should render nothing for a pending generic permission — the composer dock owns the ask", () => {
    const { container } = render(<PermissionDataPart data={basePermissionData} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("Should leave a waiting line for a pending terminal ask", () => {
    render(
      <PermissionDataPart
        data={{
          ...basePermissionData,
          title: "compozy__terminal_exec",
          timestamp: "2026-08-25T12:00:00Z",
          raw: {
            tool_id: "compozy__terminal_exec",
            tool_input: {
              command: "bun",
              args: ["add", "@xterm/xterm"],
              cwd: "~/dev/atlas-api",
              risk: "ordinary",
            },
          },
        }}
      />
    );

    const waiting = screen.getByTestId("permission-waiting-line");
    expect(waiting).toHaveAttribute("role", "status");
    expect(waiting.querySelector('[data-slot="status-dot"]')).toHaveAttribute(
      "data-tone",
      "warning"
    );
    expect(waiting).toHaveTextContent("Waiting for your approval to run");
    expect(waiting).toHaveTextContent("bun add @xterm/xterm");
    expect(waiting.querySelector("time")).toHaveAttribute("dateTime", "2026-08-25T12:00:00Z");
  });

  it("Should leave an allowed receipt with scope and mono subject", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "allow-once" }} />);

    const receipt = screen.getByTestId("permission-allowed-receipt");
    expect(receipt).toHaveAttribute("data-tone", "allowed");
    expect(receipt).toHaveAttribute("data-decision", "allow-once");
    expect(receipt).toHaveTextContent("Allowed Bash once — rm -rf /tmp/test");
  });

  it("Should phrase an allow-always receipt as a project-and-agent grant", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "allow-always" }} />);

    expect(screen.getByTestId("permission-allowed-receipt")).toHaveTextContent(
      "Allowed Bash for this project and this agent"
    );
  });

  it("Should leave a rejected receipt in the board's plain register once the daemon says you answered", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "reject-once" },
      resolvedRow("operator")
    );

    const receipt = screen.getByTestId("permission-rejected-notice");
    expect(receipt).toHaveAttribute("data-tone", "rejected");
    expect(receipt).toHaveAttribute("data-actor", "you");
    expect(receipt).toHaveTextContent("Not allowed by you · Bash — rm -rf /tmp/test");
  });

  it("Should phrase a reject-always receipt as a project-and-agent refusal", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "reject-always" },
      resolvedRow("operator:control", { resolution: "reject-always" })
    );

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Not allowed by you · Bash for this project and this agent — rm -rf /tmp/test"
    );
  });

  it("Should say a refused terminal command did not run", () => {
    renderWithResolvedRow(
      { ...terminalExecData, decision: "reject-once" },
      resolvedRow("operator")
    );

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Not allowed by you · rm -rf /var/lib/atlas/journal-backups did not run"
    );
  });

  it("Should not blame you for a refusal the daemon never attributed", () => {
    render(<PermissionDataPart data={{ ...terminalExecData, decision: "reject-once" }} />);

    const receipt = screen.getByTestId("permission-rejected-notice");
    expect(receipt).toHaveAttribute("data-actor", "unknown");
    expect(receipt).toHaveTextContent(
      "Not allowed · rm -rf /var/lib/atlas/journal-backups did not run"
    );
    expect(receipt).not.toHaveTextContent("by you");
  });

  it("Should keep a refusal neutral when the row names an actor the Web does not know", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "reject-always" },
      resolvedRow("bridge:acme", { resolution: "reject-always" })
    );

    const receipt = screen.getByTestId("permission-rejected-notice");
    expect(receipt).toHaveAttribute("data-actor", "unknown");
    expect(receipt).toHaveTextContent(
      "Not allowed · Bash for this project and this agent — rm -rf /tmp/test"
    );
  });

  it("Should attribute a timed-out ask to the timeout, never to you or the provider", () => {
    renderWithResolvedRow({ ...terminalExecData, decision: "reject-once" }, resolvedRow("timeout"));

    const receipt = screen.getByTestId("permission-rejected-notice");
    expect(receipt).toHaveAttribute("data-tone", "rejected");
    expect(receipt).toHaveAttribute("data-decision", "reject-once");
    expect(receipt).toHaveAttribute("data-actor", "timeout");
    expect(receipt).toHaveTextContent(
      "Timed out before anyone answered · rm -rf /var/lib/atlas/journal-backups did not run"
    );
    expect(receipt).not.toHaveTextContent(/by you|provider/);
  });

  it("Should name the timeout on a generic tool with its subject", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "reject-once" },
      resolvedRow("timeout")
    );

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Timed out before anyone answered · Bash — rm -rf /tmp/test"
    );
  });

  it("Should name the runtime, not you, when the row says the provider decided", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "allow-once" },
      resolvedRow("provider", { resolution: "allow-once" })
    );

    const allowed = screen.getByTestId("permission-allowed-receipt");
    expect(allowed).toHaveAttribute("data-actor", "runtime");
    expect(allowed).toHaveTextContent("Allowed by the runtime · Bash once — rm -rf /tmp/test");
    expect(allowed).not.toHaveTextContent(/asking|by you/);
  });

  it("Should attribute a provider-gated refusal to the runtime without claiming a question was shown", () => {
    renderWithResolvedRow(
      { ...terminalExecData, decision: "reject-once" },
      resolvedRow("provider")
    );

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Not allowed by the runtime · rm -rf /var/lib/atlas/journal-backups did not run"
    );
  });

  it("Should credit another agent's native approval without claiming it was you", () => {
    renderWithResolvedRow(
      { ...terminalExecData, decision: "allow-always" },
      resolvedRow("agent_session:sess-reviewer", { resolution: "allow-always" })
    );

    const allowed = screen.getByTestId("permission-allowed-receipt");
    expect(allowed).toHaveAttribute("data-actor", "agent");
    expect(allowed).toHaveTextContent(
      "Allowed by another agent · rm -rf /var/lib/atlas/journal-backups for this project and this agent"
    );
  });

  it("Should keep an allow by you in the board's bare shape", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "allow-once" },
      resolvedRow("operator", { resolution: "allow-once" })
    );

    const allowed = screen.getByTestId("permission-allowed-receipt");
    expect(allowed).toHaveAttribute("data-actor", "you");
    expect(allowed).toHaveTextContent("Allowed Bash once — rm -rf /tmp/test");
  });

  it("Should ignore a row that is not a resolved decision when attributing", () => {
    renderWithResolvedRow(
      { ...basePermissionData, decision: "reject-once" },
      resolvedRow("operator", { status: "canceled", resolution: "operator-cleared" })
    );

    expect(screen.getByTestId("permission-rejected-notice")).toHaveAttribute(
      "data-actor",
      "unknown"
    );
  });

  it("Should phrase a remembered terminal allow as a project-and-agent grant", () => {
    render(
      <PermissionDataPart
        data={{
          ...basePermissionData,
          decision: "allow-always",
          title: "compozy__terminal_exec",
          raw: {
            tool_id: "compozy__terminal_exec",
            tool_input: {
              command: "bun",
              args: ["add", "@xterm/xterm"],
              cwd: "~/dev/atlas-api",
              risk: "ordinary",
            },
          },
        }}
      />
    );

    expect(screen.getByTestId("permission-allowed-receipt")).toHaveTextContent(
      "Allowed bun add @xterm/xterm for this project and this agent"
    );
  });

  it("Should fall back to the resource when the input carries no command", () => {
    render(
      <PermissionDataPart
        data={{
          ...basePermissionData,
          decision: "allow-once",
          resource: "/workspace/notes.md",
          raw: { tool_input: {} },
        }}
      />
    );

    expect(screen.getByTestId("permission-allowed-receipt")).toHaveTextContent(
      "Allowed Bash once — /workspace/notes.md"
    );
  });
});
