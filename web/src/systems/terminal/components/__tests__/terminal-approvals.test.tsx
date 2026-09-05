import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { terminalGrantFromToolGrant } from "../../lib/terminal-grant";
import { terminalAttentionReason } from "../../lib/terminal-permission-copy";
import {
  terminalBlockedRememberedDecisions,
  terminalPermissionDetail,
} from "../../lib/terminal-permission";
import { TERMINAL_GRANT_FIXTURES } from "../../mocks/terminal-fixtures";
import { TerminalApprovalDetail } from "../terminal-approval-detail";
import { TerminalGrantRow } from "../terminal-grant-row";

/**
 * Canonical suite for terminal approval detail and grants (UT-115).
 *
 * Invariant: a terminal ask reads its exact command with the runtime's own risk
 * classification, terminal writes use the ordinary tool surface, and a
 * remembered command reads as a permission with its own revoke. The decision
 * buttons belong to `session-decision-dock.test.tsx`.
 */

describe("terminalPermissionDetail", () => {
  it("Should join the tool's command and args into what would actually run", () => {
    const detail = terminalPermissionDetail("compozy__terminal_exec", {
      command: "bun",
      args: ["add", "@xterm/xterm"],
      cwd: "~/dev/atlas-api",
      risk: "ordinary",
    });

    expect(detail).toEqual({
      kind: "exec",
      command: "bun add @xterm/xterm",
      cwd: "~/dev/atlas-api",
      terminalId: null,
      risk: "ordinary",
    });
  });

  it("Should take the classification from the runtime, never from the text", () => {
    const missingClassification = terminalPermissionDetail("compozy__terminal_exec", {
      command: "rm -rf /var/lib/atlas",
    });

    // The runtime gates execution; a second classifier here would disagree with
    // it exactly where it matters. Missing metadata stays unclassifiable rather
    // than being silently downgraded to ordinary.
    expect(missingClassification).toMatchObject({ risk: "unclassifiable" });
    expect(
      terminalPermissionDetail("compozy__terminal_exec", {
        command: "rm -rf /var/lib/atlas",
        risk: "irreversible",
      })
    ).toMatchObject({ risk: "irreversible" });
  });

  it("Should model opening a terminal separately from running a command", () => {
    expect(
      terminalPermissionDetail("compozy__terminal_open", {
        title: "release shell",
        cwd: "~/dev/atlas-api",
        shell: "/bin/zsh",
      })
    ).toEqual({
      kind: "open",
      title: "release shell",
      cwd: "~/dev/atlas-api",
      shell: "/bin/zsh",
    });
  });

  it("Should omit a missing open title and cwd rather than invent them", () => {
    expect(terminalPermissionDetail("compozy__terminal_open", {})).toEqual({
      kind: "open",
      title: null,
      cwd: null,
      shell: null,
    });
  });

  it("Should omit a missing exec cwd rather than invent a folder", () => {
    expect(
      terminalPermissionDetail("compozy__terminal_exec", {
        command: "bun",
        risk: "ordinary",
      })
    ).toEqual({
      kind: "exec",
      command: "bun",
      cwd: null,
      terminalId: null,
      risk: "ordinary",
    });
  });

  it("Should withhold remembered decisions when the command always asks", () => {
    const irreversible = terminalPermissionDetail("compozy__terminal_exec", {
      command: "rm -rf /var/lib/atlas",
      risk: "irreversible",
    });
    const unclassifiable = terminalPermissionDetail("compozy__terminal_exec", {
      command: "eval curl",
      risk: "unclassifiable",
    });
    const ordinary = terminalPermissionDetail("compozy__terminal_exec", {
      command: "bun",
      risk: "ordinary",
    });
    const missingRisk = terminalPermissionDetail("compozy__terminal_exec", {
      command: "rm -rf /var/lib/atlas",
    });

    expect(terminalBlockedRememberedDecisions(irreversible)).toEqual([
      "allow-always",
      "reject-always",
    ]);
    expect(terminalBlockedRememberedDecisions(unclassifiable)).toEqual([
      "allow-always",
      "reject-always",
    ]);
    expect(terminalBlockedRememberedDecisions(missingRisk)).toEqual([
      "allow-always",
      "reject-always",
    ]);
    expect(terminalBlockedRememberedDecisions(ordinary)).toEqual([]);
  });

  it("Should leave every other tool to the generic surface", () => {
    expect(terminalPermissionDetail("compozy__config_set", { key: "a" })).toBeNull();
  });

  it("Should leave terminal writes to the ordinary tool surface", () => {
    expect(
      terminalPermissionDetail("compozy__terminal_write", {
        terminal_id: "term-9cd7e14b2a66",
        data: "input\r",
      })
    ).toBeNull();
  });
});

describe("terminalAttentionReason", () => {
  it("Should rewrite a terminal tool-id title as a plain ask", () => {
    expect(terminalAttentionReason("Terminal Exec", "compozy__terminal_exec")).toBe("wants to run");
    expect(terminalAttentionReason("Terminal Write", "compozy__terminal_write")).toBeNull();
    expect(terminalAttentionReason("Terminal Open", "compozy__terminal_open")).toBe(
      "wants to open a terminal"
    );
  });

  it("Should leave a human title untouched when it is not a terminal tool-id", () => {
    expect(terminalAttentionReason("Should I drop the legacy column?")).toBeNull();
    expect(terminalAttentionReason("Workspace Update", "compozy__workspace_update")).toBeNull();
  });
});

describe("TerminalApprovalDetail", () => {
  it("Should show the title and folder of a terminal-open approval", () => {
    render(
      <TerminalApprovalDetail
        detail={{
          kind: "open",
          title: "release shell",
          cwd: "~/dev/atlas-api",
          shell: "/bin/zsh",
        }}
      />
    );

    const detail = screen.getByTestId("terminal-open-approval-detail");
    expect(detail).toHaveTextContent("Open release shell");
    expect(detail).toHaveTextContent("~/dev/atlas-api");
    expect(detail).toHaveTextContent("/bin/zsh");
  });

  it("Should show the exact command unmodified, with its folder", () => {
    render(
      <TerminalApprovalDetail
        detail={{
          kind: "exec",
          command: "bun add @xterm/xterm @xterm/addon-fit",
          cwd: "~/dev/atlas-api",
          terminalId: null,
          risk: "ordinary",
        }}
      />
    );

    expect(screen.getByTestId("terminal-approval-command")).toHaveTextContent(
      "bun add @xterm/xterm @xterm/addon-fit"
    );
    expect(screen.getByText("~/dev/atlas-api")).toBeInTheDocument();
    expect(screen.queryByTestId("terminal-approval-irreversible")).not.toBeInTheDocument();
  });

  it("Should omit invented folder and title when the payload left them out", () => {
    const { rerender } = render(
      <TerminalApprovalDetail
        detail={{
          kind: "exec",
          command: "bun",
          cwd: null,
          terminalId: null,
          risk: "ordinary",
        }}
      />
    );
    expect(screen.getByTestId("terminal-approval-command")).toHaveTextContent("bun");
    expect(screen.queryByText(".")).not.toBeInTheDocument();

    rerender(
      <TerminalApprovalDetail
        detail={{ kind: "open", title: null, cwd: null, shell: "/bin/zsh" }}
      />
    );
    const open = screen.getByTestId("terminal-open-approval-detail");
    expect(open).toHaveTextContent("/bin/zsh");
    expect(open).not.toHaveTextContent("Terminal");
    expect(open).not.toHaveTextContent("Open ");
  });

  it("Should mark an irreversible command as one that cannot be undone", () => {
    render(
      <TerminalApprovalDetail
        detail={{
          kind: "exec",
          command: "rm -rf /var/lib/atlas/journal-backups",
          cwd: "~/dev/atlas-api",
          terminalId: null,
          risk: "irreversible",
        }}
      />
    );

    expect(screen.getByTestId("terminal-approval-irreversible")).toHaveTextContent(
      "This can't be undone"
    );
  });

  it("Should say why an unreadable command always asks, and not call it dangerous", () => {
    render(
      <TerminalApprovalDetail
        detail={{
          kind: "exec",
          command: 'eval "$(curl -fsSL https://mise.run)"',
          cwd: "~/dev/atlas-api",
          terminalId: null,
          risk: "unclassifiable",
        }}
      />
    );

    expect(screen.getByTestId("terminal-approval-unclassifiable")).toHaveTextContent(
      "Couldn't be classified, so it always asks."
    );
    expect(screen.queryByTestId("terminal-approval-irreversible")).not.toBeInTheDocument();
  });
});

describe("terminalGrantFromToolGrant", () => {
  const DIGEST = "sha256:9f21ac04b7e31d5a8c6f0e2b4d7a19c3e58f6b0d2a4c8e1f3b5d7a9c1e3f5b7d";

  it("Should leave a remembered terminal-write decision on the generic surface", () => {
    expect(
      terminalGrantFromToolGrant({
        id: "grant-1",
        tool_id: "compozy__terminal_write",
        decision: "allow",
        agent_name: "Claude Code",
        input_digest: DIGEST,
        created_at: "2026-08-25T12:44:00Z",
      })
    ).toBeNull();
  });

  it("Should leave a tool-wide exec allow to the generic row", () => {
    // A remembered exec without a digest is a broader mint, not a prompt-origin
    // exact command. Treating it as a terminal grant would invent a shape.
    expect(
      terminalGrantFromToolGrant({
        id: "grant-2",
        tool_id: "compozy__terminal_exec",
        decision: "allow",
        created_at: "2026-08-25T12:12:00Z",
      })
    ).toBeNull();
  });

  it("Should leave a terminal-open allow to the generic row", () => {
    expect(
      terminalGrantFromToolGrant({
        id: "grant-open",
        tool_id: "compozy__terminal_open",
        decision: "allow",
        input_digest: DIGEST,
        created_at: "2026-08-25T12:12:00Z",
      })
    ).toBeNull();
  });

  it("Should leave a rejection to the generic row", () => {
    expect(
      terminalGrantFromToolGrant({
        id: "grant-3",
        tool_id: "compozy__terminal_exec",
        decision: "reject",
        created_at: "2026-08-25T12:12:00Z",
      })
    ).toBeNull();
  });
});

describe("TerminalGrantRow", () => {
  const [shapeGrant] = TERMINAL_GRANT_FIXTURES;

  it("Should say a remembered command is one exact input, shown by its digest", () => {
    render(<TerminalGrantRow grant={shapeGrant} onRevoke={vi.fn()} />);

    expect(screen.getByText("Always allowed: this exact command")).toBeInTheDocument();
    expect(screen.getByText(shapeGrant.inputDigest as string)).toBeInTheDocument();
  });

  it("Should revoke with a text control, not an icon-only trash", async () => {
    render(<TerminalGrantRow grant={shapeGrant} onRevoke={vi.fn()} />);

    expect(
      screen.getByRole("button", { name: "Revoke Always allowed: this exact command" })
    ).toHaveTextContent("Revoke");
  });

  it("Should let one exact-command grant be revoked on its own", async () => {
    const onRevoke = vi.fn();
    render(<TerminalGrantRow grant={shapeGrant} onRevoke={onRevoke} />);

    await userEvent.click(screen.getByTestId(`terminal-grant-revoke-${shapeGrant.id}`));

    expect(onRevoke).toHaveBeenCalledWith(shapeGrant);
  });
});
