import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import type { CompozyPermissionData } from "../../types";
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

  it("Should leave a rejected receipt in the board's plain register", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "reject-once" }} />);

    const receipt = screen.getByTestId("permission-rejected-notice");
    expect(receipt).toHaveAttribute("data-tone", "rejected");
    expect(receipt).toHaveTextContent("Not allowed by you · Bash — rm -rf /tmp/test");
  });

  it("Should phrase a reject-always receipt as a project-and-agent refusal", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "reject-always" }} />);

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Not allowed by you · Bash for this project and this agent — rm -rf /tmp/test"
    );
  });

  it("Should say a refused terminal command did not run", () => {
    render(
      <PermissionDataPart
        data={{
          ...basePermissionData,
          decision: "reject-once",
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
        }}
      />
    );

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Not allowed by you · rm -rf /var/lib/atlas/journal-backups did not run"
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
