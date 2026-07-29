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
  it("Should render nothing for a pending permission — the composer dock owns the ask", () => {
    const { container } = render(<PermissionDataPart data={basePermissionData} />);
    expect(container).toBeEmptyDOMElement();
  });

  it("Should leave an allowed receipt with scope and mono subject", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "allow-once" }} />);

    const receipt = screen.getByTestId("permission-allowed-receipt");
    expect(receipt).toHaveAttribute("data-tone", "allowed");
    expect(receipt).toHaveAttribute("data-decision", "allow-once");
    expect(receipt).toHaveTextContent("Allowed Bash once — rm -rf /tmp/test");
  });

  it("Should phrase an allow-always receipt as a session-scope grant", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "allow-always" }} />);

    expect(screen.getByTestId("permission-allowed-receipt")).toHaveTextContent(
      "Allowed Bash for this session"
    );
  });

  it("Should leave a rejected receipt with the danger glyph tone", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "reject-once" }} />);

    const receipt = screen.getByTestId("permission-rejected-notice");
    expect(receipt).toHaveAttribute("data-tone", "rejected");
    expect(receipt).toHaveTextContent("Rejected Bash — rm -rf /tmp/test");
  });

  it("Should phrase a reject-always receipt as a session-scope rejection", () => {
    render(<PermissionDataPart data={{ ...basePermissionData, decision: "reject-always" }} />);

    expect(screen.getByTestId("permission-rejected-notice")).toHaveTextContent(
      "Rejected Bash for this session"
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
